package goamx

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pawnkit/goamx/vm"
	"github.com/pawnkit/pawndebug/debug"
)

func TestLaunchStopInspectAndContinue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "program.amx")
	if err := os.WriteFile(path, testAMX(vm.OP_CONST_PRI, 42, vm.OP_HALT, 0), 0o644); err != nil {
		t.Fatal(err)
	}
	backend := New()
	if err := backend.Launch(path); err != nil {
		t.Fatal(err)
	}
	if frames := backend.Frames(); len(frames) != 1 || frames[0].Name != "AMX" {
		t.Fatalf("frames = %+v", frames)
	}
	if value, err := backend.Evaluate("PRI"); err != nil || value != "0" {
		t.Fatalf("PRI = %q, %v", value, err)
	}
	if err := backend.Continue(); !errors.Is(err, debug.ErrExited) {
		t.Fatalf("Continue() = %v", err)
	}
	if frames := backend.Frames(); len(frames) != 0 {
		t.Fatalf("frames after exit = %+v", frames)
	}
	if err := backend.Disconnect(); err != nil {
		t.Fatal(err)
	}
}

func TestInspectionRequiresStoppedProgram(t *testing.T) {
	backend := New()

	if _, err := backend.Evaluate("PRI"); err == nil {
		t.Fatal("evaluation succeeded before launch")
	}

	if variables := backend.Variables(1); variables != nil {
		t.Fatalf("variables = %#v", variables)
	}
}

func TestSourceBreakpointStopsAtDebugLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.amx")
	if err := os.WriteFile(path, debugAMX(), 0o644); err != nil {
		t.Fatal(err)
	}
	backend := New()
	if err := backend.Launch(path); err != nil {
		t.Fatal(err)
	}
	breakpoints := backend.Breakpoints("test.pwn", []int{12, 99})
	if len(breakpoints) != 2 || !breakpoints[0].Verified || breakpoints[1].Verified {
		t.Fatalf("breakpoints = %+v", breakpoints)
	}
	if err := backend.Continue(); err != nil {
		t.Fatalf("Continue() = %v", err)
	}
	frames := backend.Frames()
	if len(frames) != 1 || frames[0].Source != "test.pwn" || frames[0].Line != 12 || frames[0].Name != "main" {
		t.Fatalf("frames = %+v", frames)
	}
	variables := backend.Variables(1)
	found := false
	for _, variable := range variables {
		if variable.Name == "score" && variable.Value == "0" {
			found = true
		}
	}
	if !found {
		t.Fatalf("variables = %+v", variables)
	}
	var arrayReference int
	for _, variable := range variables {
		if variable.Name == "values" && variable.Value == "array[3]" {
			arrayReference = variable.Reference
		}
	}
	if arrayReference == 0 {
		t.Fatalf("array variable missing: %+v", variables)
	}
	items := backend.Variables(arrayReference)
	if len(items) != 3 || items[0].Name != "[0]" || items[2].Value != "0" {
		t.Fatalf("array values = %+v", items)
	}
}

func debugAMX() []byte {
	data := testAMX(vm.OP_CONST_PRI, 42, vm.OP_HALT, 0)
	binary.LittleEndian.PutUint16(data[8:10], 0x0002)
	chunk := make([]byte, 22)
	binary.LittleEndian.PutUint16(chunk[4:6], 0xf1ef)
	chunk[6], chunk[7] = 8, 11
	binary.LittleEndian.PutUint16(chunk[10:12], 1)
	binary.LittleEndian.PutUint16(chunk[12:14], 1)
	binary.LittleEndian.PutUint16(chunk[14:16], 3)
	append32 := func(value uint32) {
		var raw [4]byte
		binary.LittleEndian.PutUint32(raw[:], value)
		chunk = append(chunk, raw[:]...)
	}
	append16 := func(value uint16) {
		var raw [2]byte
		binary.LittleEndian.PutUint16(raw[:], value)
		chunk = append(chunk, raw[:]...)
	}
	appendName := func(value string) { chunk = append(chunk, value...); chunk = append(chunk, 0) }
	append32(0)
	appendName("test.pwn")
	append32(0)
	append32(12)
	append32(0)
	append16(0)
	append32(0)
	append32(16)
	chunk = append(chunk, 9, 0)
	append16(0)
	appendName("main")
	append32(0)
	append16(0)
	append32(0)
	append32(16)
	chunk = append(chunk, 1, 0)
	append16(0)
	appendName("score")
	append32(4)
	append16(0)
	append32(0)
	append32(16)
	chunk = append(chunk, 3, 0)
	append16(1)
	appendName("values")
	append16(0)
	append32(3)
	binary.LittleEndian.PutUint32(chunk[0:4], uint32(len(chunk)))
	return append(data, chunk...)
}

func testAMX(cells ...any) []byte {
	const headerSize = 56
	publics, natives := uint32(headerSize), uint32(headerSize+8)
	data := make([]byte, natives)
	data = append(data, 31, 0)
	binary.LittleEndian.PutUint32(data[publics+4:publics+8], uint32(len(data)))
	data = append(data, "main"...)
	data = append(data, 0)
	for len(data)%4 != 0 {
		data = append(data, 0)
	}
	code := uint32(len(data))
	for _, item := range cells {
		value := int32(0)
		switch item := item.(type) {
		case vm.Opcode:
			value = int32(item)
		case int:
			value = int32(item)
		}
		var cell [4]byte
		binary.LittleEndian.PutUint32(cell[:], uint32(value))
		data = append(data, cell[:]...)
	}
	dat := uint32(len(data))
	binary.LittleEndian.PutUint32(data[0:4], dat)
	binary.LittleEndian.PutUint16(data[4:6], 0xf1e0)
	data[6], data[7] = 8, 11
	binary.LittleEndian.PutUint16(data[10:12], 8)
	binary.LittleEndian.PutUint32(data[12:16], code)
	binary.LittleEndian.PutUint32(data[16:20], dat)
	binary.LittleEndian.PutUint32(data[20:24], dat)
	binary.LittleEndian.PutUint32(data[24:28], dat+256)
	binary.LittleEndian.PutUint32(data[32:36], publics)
	binary.LittleEndian.PutUint32(data[36:40], natives)
	for offset := 40; offset <= 52; offset += 4 {
		binary.LittleEndian.PutUint32(data[offset:offset+4], natives)
	}
	return data
}
