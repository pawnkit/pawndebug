package goamx

import (
	"errors"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	amx "github.com/pawnkit/goamx"
	"github.com/pawnkit/pawndebug/debug"
)

// Backend debugs one goamx runtime.
type Backend struct {
	mu          sync.Mutex
	runtime     *amx.Runtime
	breakpoints map[string]map[int]bool
	location    debug.Frame
	state       amx.State
	step        bool
	skipOffset  int
	arrays      map[int]arrayVariable
	nextArray   int
}

type arrayVariable struct {
	address    amx.Cell
	dimensions []int
}

// New creates an empty backend.
func New() *Backend {
	return &Backend{breakpoints: map[string]map[int]bool{}, skipOffset: -1, arrays: map[int]arrayVariable{}, nextArray: 2}
}

// Launch loads an AMX file and stops at its first instruction.
func (backend *Backend) Launch(path string) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()

	if backend.runtime != nil {
		return errors.New("a program is already loaded")
	}

	runtime, err := amx.LoadFile(path)
	if err != nil {
		return err
	}

	backend.runtime = runtime
	backend.step = true
	backend.skipOffset = -1
	runtime.SetDebugHook(backend.hook)

	if runtime.Info().HasMain {
		_, err = runtime.ExecMain()
	} else {
		publics, publicErr := runtime.Publics()
		if publicErr != nil {
			err = publicErr
		} else if len(publics) == 0 {
			err = errors.New("AMX has no entry point")
		} else {
			_, err = runtime.ExecPublic(publics[0].Index)
		}
	}
	if errors.Is(err, amx.ErrExecutionPaused) {
		return nil
	}

	if err == nil {
		err = errors.New("program exited before the debugger stopped")
	}

	closeErr := runtime.Close()
	backend.runtime = nil

	return errors.Join(err, closeErr)
}

func (backend *Backend) hook(event amx.DebugEvent) error {
	if int(event.Instruction.Offset) == backend.skipOffset {
		backend.skipOffset = -1
		return nil
	}
	file, line, function, _ := backend.runtime.DebugLocation(amx.Cell(event.Instruction.Offset))
	if backend.step || backend.breakpoints[canonicalPath(file)][line] {
		backend.step = false
		backend.state = event.State
		backend.location = debug.Frame{ID: 1, Name: function, Source: file, Line: line}
		backend.arrays = map[int]arrayVariable{}
		backend.nextArray = 2
		if function == "" {
			backend.location.Name = "AMX"
		}

		if line < 1 {
			backend.location.Line = 1
		}

		return amx.ErrExecutionPaused
	}
	return nil
}

// Breakpoints replaces the breakpoints for a source file.
func (backend *Backend) Breakpoints(source string, lines []int) []debug.Breakpoint {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	resolved := map[int]bool{}
	available := map[int]bool{}
	if backend.runtime != nil {
		info := backend.runtime.DebugInfo()
		for _, entry := range info.Lines {
			file, ok := info.FileAt(entry.Address)
			if ok && canonicalPath(file.Name) == canonicalPath(source) {
				available[int(entry.Line)] = true
			}
		}
	}
	result := make([]debug.Breakpoint, 0, len(lines))
	for _, line := range lines {
		verified := available[line]
		if verified {
			resolved[line] = true
		}
		result = append(result, debug.Breakpoint{Source: source, Line: line, Verified: verified})
	}
	backend.breakpoints[canonicalPath(source)] = resolved
	return result
}

// Continue resumes execution.
func (backend *Backend) Continue() error { return backend.resume(false) }

// Step advances one instruction.
func (backend *Backend) Step(string) error { return backend.resume(true) }

func (backend *Backend) resume(step bool) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.runtime == nil || !backend.runtime.Suspended() {
		return errors.New("program is not stopped")
	}
	backend.step = step
	backend.skipOffset = backend.state.CIP
	_, err := backend.runtime.Continue()
	if errors.Is(err, amx.ErrExecutionPaused) {
		return nil
	}
	if err == nil {
		return debug.ErrExited
	}
	return err
}

// Frames returns the current stopped frame.
func (backend *Backend) Frames() []debug.Frame {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.runtime == nil || !backend.runtime.Suspended() {
		return nil
	}
	return []debug.Frame{backend.location}
}

// Variables returns values for a DAP variable reference.
func (backend *Backend) Variables(reference int) []debug.Variable {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.runtime == nil || !backend.runtime.Suspended() {
		return nil
	}
	if array, ok := backend.arrays[reference]; ok {
		return backend.arrayValues(array)
	}
	if reference != 1 {
		return nil
	}
	backend.arrays = map[int]arrayVariable{}
	backend.nextArray = 2
	values := []struct {
		name  string
		value amx.Cell
	}{{"PRI", backend.state.PRI}, {"ALT", backend.state.ALT}, {"HEA", backend.state.HEA}, {"STK", backend.state.STK}, {"FRM", backend.state.FRM}}
	result := make([]debug.Variable, 0, len(values))
	for _, item := range values {
		result = append(result, debug.Variable{Name: item.name, Value: strconv.FormatInt(int64(item.value), 10)})
	}
	for _, symbol := range backend.runtime.DebugInfo().Symbols {
		if symbol.Ident != amx.SymbolVariable && symbol.Ident != amx.SymbolReference && symbol.Ident != amx.SymbolArray && symbol.Ident != amx.SymbolRefArray {
			continue
		}
		cip := int64(backend.state.CIP)
		if symbol.CodeEnd != 0 && (cip < int64(symbol.CodeStart) || cip >= int64(symbol.CodeEnd)) {
			continue
		}

		address, ok := backend.symbolAddress(symbol)
		if !ok {
			continue
		}
		if symbol.Ident == amx.SymbolArray || symbol.Ident == amx.SymbolRefArray {
			dimensions, ok := arrayDimensions(symbol.Dimensions)
			if !ok {
				continue
			}
			reference := backend.nextArray
			backend.nextArray++
			backend.arrays[reference] = arrayVariable{address: address, dimensions: dimensions}
			result = append(result, debug.Variable{Name: symbol.Name, Value: arrayType(dimensions), Type: "array", Reference: reference})
			continue
		}
		value, err := backend.runtime.ReadCell(address)
		if err == nil {
			result = append(result, debug.Variable{Name: symbol.Name, Value: strconv.FormatInt(int64(value), 10)})
		}
	}
	return result
}

func (backend *Backend) symbolAddress(symbol amx.DebugSymbol) (amx.Cell, bool) {
	if symbol.Address > math.MaxInt32 {
		return 0, false
	}
	address := amx.Cell(symbol.Address)
	if symbol.Class == 1 {
		address = backend.state.FRM + address
	}
	if symbol.Ident == amx.SymbolReference || symbol.Ident == amx.SymbolRefArray {
		value, err := backend.runtime.ReadCell(address)
		return value, err == nil
	}
	return address, true
}

func (backend *Backend) arrayValues(array arrayVariable) []debug.Variable {
	if len(array.dimensions) == 0 {
		return nil
	}
	stride := 1
	for _, size := range array.dimensions[1:] {
		stride *= size
	}
	result := make([]debug.Variable, 0, array.dimensions[0])
	for index := range array.dimensions[0] {
		name := "[" + strconv.Itoa(index) + "]"
		address := array.address + amx.Cell(index*stride*amx.CellBytes) //nolint:gosec // arrayDimensions caps this offset.
		if len(array.dimensions) > 1 {
			reference := backend.nextArray
			backend.nextArray++
			child := arrayVariable{address: address, dimensions: append([]int(nil), array.dimensions[1:]...)}
			backend.arrays[reference] = child
			result = append(result, debug.Variable{Name: name, Value: arrayType(child.dimensions), Type: "array", Reference: reference})
			continue
		}
		value, err := backend.runtime.ReadCell(address)
		if err != nil {
			break
		}
		result = append(result, debug.Variable{Name: name, Value: strconv.FormatInt(int64(value), 10)})
	}
	return result
}

func arrayDimensions(dimensions []amx.DebugDimension) ([]int, bool) {
	if len(dimensions) == 0 {
		return nil, false
	}
	result := make([]int, len(dimensions))
	total := 1
	for index, dimension := range dimensions {
		if dimension.Size == 0 || dimension.Size > 4096 || total > 4096/int(dimension.Size) {
			return nil, false
		}
		result[index] = int(dimension.Size)
		total *= result[index]
	}
	return result, true
}

func arrayType(dimensions []int) string {
	var result strings.Builder
	result.WriteString("array")
	for _, size := range dimensions {
		result.WriteByte('[')
		result.WriteString(strconv.Itoa(size))
		result.WriteByte(']')
	}
	return result.String()
}

// Evaluate reads a supported register.
func (backend *Backend) Evaluate(expression string) (string, error) {
	backend.mu.Lock()
	defer backend.mu.Unlock()

	if backend.runtime == nil || !backend.runtime.Suspended() {
		return "", errors.New("program is not stopped")
	}

	values := map[string]amx.Cell{"pri": backend.state.PRI, "alt": backend.state.ALT, "hea": backend.state.HEA, "stk": backend.state.STK, "frm": backend.state.FRM}
	value, ok := values[strings.ToLower(strings.TrimSpace(expression))]
	if !ok {
		return "", errors.New("only PRI, ALT, HEA, STK, and FRM can be evaluated")
	}
	return strconv.FormatInt(int64(value), 10), nil
}

// Disconnect closes the runtime.
func (backend *Backend) Disconnect() error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.runtime == nil {
		return nil
	}
	err := backend.runtime.Close()
	backend.runtime = nil
	backend.location = debug.Frame{}
	backend.state = amx.State{}
	backend.step = false
	backend.skipOffset = -1
	backend.arrays = map[int]arrayVariable{}
	backend.nextArray = 2

	return err
}

func canonicalPath(path string) string {
	absolute, err := filepath.Abs(path)
	if err == nil {
		path = absolute
	}

	return filepath.ToSlash(filepath.Clean(path))
}
