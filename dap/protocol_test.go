package dap

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestFraming(t *testing.T) {
	var output bytes.Buffer
	message := Message{Seq: 1, Type: "request", Command: "initialize"}
	if err := Write(&output, message); err != nil {
		t.Fatal(err)
	}
	got, err := Read(bufio.NewReader(&output))
	if err != nil || got.Command != "initialize" {
		t.Fatalf("got %#v, %v", got, err)
	}
}

func TestFailureResponseIncludesSuccess(t *testing.T) {
	var output bytes.Buffer
	succeeded := false
	message := Message{Seq: 1, Type: "response", RequestSeq: 1, Success: &succeeded, Message: "failed"}

	if err := Write(&output, message); err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(output.Bytes(), []byte(`"success":false`)) {
		t.Fatalf("response = %q", output.String())
	}
}

func TestRejectsOversize(t *testing.T) {
	_, err := Read(bufio.NewReader(strings.NewReader("Content-Length: 999999999\r\n\r\n")))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRejectsDuplicateLength(t *testing.T) {
	input := "Content-Length: 2\r\nContent-Length: 2\r\n\r\n{}"
	if _, err := Read(bufio.NewReader(strings.NewReader(input))); err == nil {
		t.Fatal("duplicate Content-Length was accepted")
	}
}

func TestRejectsLongHeader(t *testing.T) {
	input := strings.Repeat("x", 5000) + "\r\n\r\n"
	if _, err := Read(bufio.NewReaderSize(strings.NewReader(input), 128)); err == nil {
		t.Fatal("long header was accepted")
	}
}
