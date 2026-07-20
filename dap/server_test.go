package dap

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"

	"github.com/pawnkit/pawndebug/debug"
)

type stubBackend struct {
	launched bool
}

func (backend *stubBackend) Launch(string) error {
	backend.launched = true

	return nil
}

func (*stubBackend) Breakpoints(source string, lines []int) []debug.Breakpoint {
	return []debug.Breakpoint{{Source: source, Line: lines[0], Verified: true}}
}

func (*stubBackend) Continue() error                 { return nil }
func (*stubBackend) Step(string) error               { return nil }
func (*stubBackend) Frames() []debug.Frame           { return nil }
func (*stubBackend) Variables(int) []debug.Variable  { return nil }
func (*stubBackend) Evaluate(string) (string, error) { return "", nil }
func (*stubBackend) Disconnect() error               { return nil }

func TestLaunchRejectsInvalidArguments(t *testing.T) {
	backend := &stubBackend{}
	response := handleRequest(t, backend, Message{
		Seq:       1,
		Type:      "request",
		Command:   "launch",
		Arguments: json.RawMessage(`{"program":`),
	})

	if response.Success == nil || *response.Success || backend.launched || response.Message == "" {
		t.Fatalf("response = %#v, launched = %v", response, backend.launched)
	}
}

func TestSetBreakpointsUsesDAPSourceObject(t *testing.T) {
	arguments := json.RawMessage(`{"source":{"path":"test.pwn"},"breakpoints":[{"line":12}]}`)
	response := handleRequest(t, &stubBackend{}, Message{Seq: 1, Type: "request", Command: "setBreakpoints", Arguments: arguments})

	body, err := json.Marshal(response.Body)
	if err != nil {
		t.Fatal(err)
	}

	var result struct {
		Breakpoints []struct {
			Source struct {
				Path string `json:"path"`
			} `json:"source"`
			Line     int  `json:"line"`
			Verified bool `json:"verified"`
		} `json:"breakpoints"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}

	if len(result.Breakpoints) != 1 || result.Breakpoints[0].Source.Path != "test.pwn" || !result.Breakpoints[0].Verified {
		t.Fatalf("body = %s", body)
	}
}

func TestPauseIsUnsupported(t *testing.T) {
	response := handleRequest(t, &stubBackend{}, Message{Seq: 1, Type: "request", Command: "pause"})
	if response.Success == nil || *response.Success || response.Message == "" {
		t.Fatalf("response = %#v", response)
	}
}

func handleRequest(t *testing.T, backend debug.Backend, request Message) Message {
	t.Helper()

	var output bytes.Buffer
	server := &Server{Backend: backend}

	if _, err := server.handle(&output, request); err != nil {
		t.Fatal(err)
	}

	response, err := Read(bufio.NewReader(&output))
	if err != nil {
		t.Fatal(err)
	}

	return response
}
