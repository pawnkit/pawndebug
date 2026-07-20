package dap

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	maxMessage = 8 << 20
	maxHeader  = 8 << 10
)

// Message contains fields shared by DAP messages.
type Message struct {
	Seq        int             `json:"seq"`
	Type       string          `json:"type"`
	Command    string          `json:"command,omitempty"`
	RequestSeq int             `json:"request_seq,omitempty"`
	Success    *bool           `json:"success,omitempty"`
	Event      string          `json:"event,omitempty"`
	Arguments  json.RawMessage `json:"arguments,omitempty"`
	Body       any             `json:"body,omitempty"`
	Message    string          `json:"message,omitempty"`
}

// Read decodes one framed DAP message.
func Read(reader *bufio.Reader) (Message, error) {
	length := -1
	headerBytes := 0

	for {
		line, err := reader.ReadSlice('\n')
		if err != nil {
			if errors.Is(err, bufio.ErrBufferFull) {
				return Message{}, errors.New("DAP header line is too long")
			}

			return Message{}, err
		}

		headerBytes += len(line)
		if headerBytes > maxHeader {
			return Message{}, errors.New("DAP headers are too large")
		}

		text := strings.TrimSpace(string(line))
		if text == "" {
			break
		}

		key, value, ok := strings.Cut(text, ":")
		if ok && strings.EqualFold(key, "Content-Length") {
			if length >= 0 {
				return Message{}, errors.New("duplicate Content-Length header")
			}

			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return Message{}, err
			}
		}
	}

	if length <= 0 || length > maxMessage {
		return Message{}, errors.New("invalid Content-Length header")
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(reader, data); err != nil {
		return Message{}, err
	}
	var message Message

	if err := json.Unmarshal(data, &message); err != nil {
		return Message{}, err
	}
	return message, nil
}

// Write encodes one framed DAP message.
func Write(writer io.Writer, message Message) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n%s", len(data), data)
	return err
}
