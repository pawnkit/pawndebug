package debug

import "errors"

// ErrExited reports normal program completion.
var ErrExited = errors.New("program exited")

// Breakpoint is a resolved source breakpoint.
type Breakpoint struct {
	Source   string `json:"source"`
	Line     int    `json:"line"`
	Verified bool   `json:"verified"`
}

// Frame describes a stopped Pawn frame.
type Frame struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
	Line   int    `json:"line"`
}

// Variable is a value shown by the debugger.
type Variable struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Reference int    `json:"variablesReference"`
}

// Backend controls one debug runtime.
type Backend interface {
	Launch(path string) error
	Breakpoints(source string, lines []int) []Breakpoint
	Continue() error
	Step(kind string) error
	Frames() []Frame
	Variables(reference int) []Variable
	Evaluate(expression string) (string, error)
	Disconnect() error
}
