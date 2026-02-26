package sse

import (
	"bufio"
	"io"
	"strings"
)

// Event represents a single Server-Sent Event.
type Event struct {
	Type string // e.g., "deployment_event", "build_log", "runtime_log"
	Data string // raw JSON string
	ID   string // optional event ID
}

// Reader parses a text/event-stream response body.
type Reader struct {
	scanner *bufio.Scanner
	closer  io.Closer
}

// NewReader creates an SSE reader from a response body.
func NewReader(body io.ReadCloser) *Reader {
	return &Reader{
		scanner: bufio.NewScanner(body),
		closer:  body,
	}
}

// Next reads the next SSE event from the stream.
// Returns io.EOF when the stream ends.
func (r *Reader) Next() (*Event, error) {
	var eventType string
	var dataLines []string
	var id string

	for r.scanner.Scan() {
		line := r.scanner.Text()

		// Empty line = event boundary.
		if line == "" {
			if len(dataLines) > 0 {
				evt := &Event{
					Type: eventType,
					Data: strings.Join(dataLines, "\n"),
					ID:   id,
				}
				if evt.Type == "" {
					evt.Type = "message"
				}
				return evt, nil
			}
			// Reset for next event.
			eventType = ""
			dataLines = nil
			id = ""
			continue
		}

		// Comment line — ignore.
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Parse field: value
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")

		switch field {
		case "event":
			eventType = value
		case "data":
			dataLines = append(dataLines, value)
		case "id":
			id = value
		case "retry":
			// Ignored — CLI doesn't auto-reconnect at the reader level.
		}
	}

	if err := r.scanner.Err(); err != nil {
		return nil, err
	}

	// If we have pending data when the stream ends, emit it.
	if len(dataLines) > 0 {
		evt := &Event{
			Type: eventType,
			Data: strings.Join(dataLines, "\n"),
			ID:   id,
		}
		if evt.Type == "" {
			evt.Type = "message"
		}
		return evt, nil
	}

	return nil, io.EOF
}

// Close closes the underlying stream.
func (r *Reader) Close() error {
	return r.closer.Close()
}
