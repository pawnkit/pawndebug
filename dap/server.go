package dap

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/pawnkit/pawndebug/debug"
)

// Server handles DAP requests for one backend.
type Server struct {
	Backend debug.Backend
	seq     int
}

// Serve processes requests until disconnect or EOF.
func (server *Server) Serve(input io.Reader, output io.Writer) error {
	if server.Backend == nil {
		return errors.New("debug backend is unavailable")
	}

	reader := bufio.NewReader(input)

	for {
		request, err := Read(reader)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		if err := validateRequest(request); err != nil {
			if responseErr := server.response(output, request, nil, err); responseErr != nil {
				return errors.Join(err, responseErr)
			}

			continue
		}

		done, err := server.handle(output, request)
		if err != nil {
			return err
		}

		if done {
			return nil
		}
	}
}

func validateRequest(request Message) error {
	if request.Type != "request" {
		return fmt.Errorf("expected request, received %q", request.Type)
	}

	if request.Seq <= 0 {
		return errors.New("request sequence must be positive")
	}

	if request.Command == "" {
		return errors.New("request command is missing")
	}

	return nil
}

func (server *Server) response(output io.Writer, request Message, body any, err error) error {
	server.seq++
	succeeded := err == nil
	response := Message{Seq: server.seq, Type: "response", Command: request.Command, RequestSeq: request.Seq, Success: &succeeded, Body: body}
	if err != nil {
		response.Message = err.Error()
	}
	return Write(output, response)
}

func (server *Server) event(output io.Writer, name string, body any) error {
	server.seq++
	return Write(output, Message{Seq: server.seq, Type: "event", Event: name, Body: body})
}

func (server *Server) handle(output io.Writer, request Message) (bool, error) {
	switch request.Command {
	case "initialize":
		if err := server.response(output, request, map[string]bool{"supportsConfigurationDoneRequest": true, "supportsEvaluateForHovers": true, "supportsDisassembleRequest": false, "supportsReadMemoryRequest": false}, nil); err != nil {
			return false, err
		}
		return false, server.event(output, "initialized", nil)
	case "launch":
		var args struct {
			Program string `json:"program"`
		}
		if err := json.Unmarshal(request.Arguments, &args); err != nil {
			return false, server.response(output, request, nil, fmt.Errorf("invalid launch arguments: %w", err))
		}

		if args.Program == "" {
			return false, server.response(output, request, nil, errors.New("launch requires a program path"))
		}

		err := server.Backend.Launch(args.Program)
		if responseErr := server.response(output, request, nil, err); responseErr != nil || err != nil {
			return false, responseErr
		}
		return false, server.event(output, "stopped", map[string]any{"reason": "entry", "threadId": 1, "allThreadsStopped": true})
	case "threads":
		return false, server.response(output, request, map[string]any{"threads": []map[string]any{{"id": 1, "name": "Pawn runtime"}}}, nil)
	case "continue":
		err := server.Backend.Continue()
		if errors.Is(err, debug.ErrExited) {
			if responseErr := server.response(output, request, map[string]bool{"allThreadsContinued": true}, nil); responseErr != nil {
				return false, responseErr
			}
			return false, server.event(output, "terminated", nil)
		}
		if responseErr := server.response(output, request, map[string]bool{"allThreadsContinued": true}, err); responseErr != nil || err != nil {
			return false, responseErr
		}
		return false, server.event(output, "stopped", map[string]any{"reason": "breakpoint", "threadId": 1, "allThreadsStopped": true})
	case "next", "stepIn", "stepOut":
		err := server.Backend.Step(request.Command)
		if errors.Is(err, debug.ErrExited) {
			if responseErr := server.response(output, request, nil, nil); responseErr != nil {
				return false, responseErr
			}
			return false, server.event(output, "terminated", nil)
		}
		if responseErr := server.response(output, request, nil, err); responseErr != nil || err != nil {
			return false, responseErr
		}
		return false, server.event(output, "stopped", map[string]any{"reason": "step", "threadId": 1, "allThreadsStopped": true})
	case "stackTrace":
		frames := server.Backend.Frames()
		out := make([]map[string]any, 0, len(frames))
		for _, frame := range frames {
			item := map[string]any{"id": frame.ID, "name": frame.Name, "line": frame.Line, "column": 1}
			if frame.Source != "" {
				item["source"] = map[string]any{"name": filepath.Base(frame.Source), "path": frame.Source}
			}
			out = append(out, item)
		}
		return false, server.response(output, request, map[string]any{"stackFrames": out, "totalFrames": len(out)}, nil)
	case "scopes":
		return false, server.response(output, request, map[string]any{"scopes": []map[string]any{{"name": "Pawn", "variablesReference": 1, "expensive": false}}}, nil)
	case "variables":
		var args struct {
			Reference int `json:"variablesReference"`
		}
		if err := json.Unmarshal(request.Arguments, &args); err != nil {
			return false, server.response(output, request, nil, fmt.Errorf("invalid variables arguments: %w", err))
		}

		return false, server.response(output, request, map[string]any{"variables": server.Backend.Variables(args.Reference)}, nil)
	case "evaluate":
		var args struct {
			Expression string `json:"expression"`
		}
		if err := json.Unmarshal(request.Arguments, &args); err != nil {
			return false, server.response(output, request, nil, fmt.Errorf("invalid evaluate arguments: %w", err))
		}

		value, err := server.Backend.Evaluate(args.Expression)
		return false, server.response(output, request, map[string]any{"result": value, "variablesReference": 0}, err)
	case "disconnect", "terminate":
		err := server.Backend.Disconnect()
		return true, server.response(output, request, nil, err)
	case "setBreakpoints":
		var args struct {
			Source struct {
				Path string `json:"path"`
			} `json:"source"`
			Breakpoints []struct {
				Line int `json:"line"`
			} `json:"breakpoints"`
		}
		if err := json.Unmarshal(request.Arguments, &args); err != nil {
			return false, server.response(output, request, nil, err)
		}
		lines := make([]int, len(args.Breakpoints))
		for i, breakpoint := range args.Breakpoints {
			lines[i] = breakpoint.Line
		}
		breakpoints := server.Backend.Breakpoints(args.Source.Path, lines)
		outputBreakpoints := make([]map[string]any, 0, len(breakpoints))

		for _, breakpoint := range breakpoints {
			item := map[string]any{"line": breakpoint.Line, "verified": breakpoint.Verified}
			if breakpoint.Source != "" {
				item["source"] = map[string]any{"name": filepath.Base(breakpoint.Source), "path": breakpoint.Source}
			}

			outputBreakpoints = append(outputBreakpoints, item)
		}

		return false, server.response(output, request, map[string]any{"breakpoints": outputBreakpoints}, nil)
	case "configurationDone":
		return false, server.response(output, request, nil, nil)
	default:
		return false, server.response(output, request, nil, fmt.Errorf("request %q is unsupported", request.Command))
	}
}
