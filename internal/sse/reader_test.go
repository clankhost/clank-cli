package sse

import (
	"io"
	"strings"
	"testing"
)

// nopCloser wraps a Reader to implement ReadCloser.
type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func TestReaderBasicEvent(t *testing.T) {
	stream := "event: deployment_event\ndata: {\"event_type\":\"deployment.active\",\"message\":\"Deployed\"}\n\n"
	reader := NewReader(nopCloser{strings.NewReader(stream)})

	evt, err := reader.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "deployment_event" {
		t.Errorf("expected type 'deployment_event', got %q", evt.Type)
	}
	if evt.Data != `{"event_type":"deployment.active","message":"Deployed"}` {
		t.Errorf("unexpected data: %s", evt.Data)
	}

	// Next call should return EOF.
	_, err = reader.Next()
	if err != io.EOF {
		t.Errorf("expected EOF, got: %v", err)
	}
}

func TestReaderMultiLineData(t *testing.T) {
	stream := "event: build_log\ndata: line 1\ndata: line 2\ndata: line 3\n\n"
	reader := NewReader(nopCloser{strings.NewReader(stream)})

	evt, err := reader.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Data != "line 1\nline 2\nline 3" {
		t.Errorf("unexpected data: %q", evt.Data)
	}
}

func TestReaderIgnoresComments(t *testing.T) {
	stream := ": this is a comment\nevent: test\ndata: hello\n\n"
	reader := NewReader(nopCloser{strings.NewReader(stream)})

	evt, err := reader.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "test" {
		t.Errorf("expected type 'test', got %q", evt.Type)
	}
	if evt.Data != "hello" {
		t.Errorf("expected data 'hello', got %q", evt.Data)
	}
}

func TestReaderDefaultEventType(t *testing.T) {
	stream := "data: no event type\n\n"
	reader := NewReader(nopCloser{strings.NewReader(stream)})

	evt, err := reader.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "message" {
		t.Errorf("expected default type 'message', got %q", evt.Type)
	}
}

func TestReaderMultipleEvents(t *testing.T) {
	stream := "event: a\ndata: first\n\nevent: b\ndata: second\n\n"
	reader := NewReader(nopCloser{strings.NewReader(stream)})

	evt1, err := reader.Next()
	if err != nil {
		t.Fatalf("event 1 error: %v", err)
	}
	if evt1.Type != "a" || evt1.Data != "first" {
		t.Errorf("event 1: type=%q data=%q", evt1.Type, evt1.Data)
	}

	evt2, err := reader.Next()
	if err != nil {
		t.Fatalf("event 2 error: %v", err)
	}
	if evt2.Type != "b" || evt2.Data != "second" {
		t.Errorf("event 2: type=%q data=%q", evt2.Type, evt2.Data)
	}

	_, err = reader.Next()
	if err != io.EOF {
		t.Errorf("expected EOF, got: %v", err)
	}
}

func TestReaderWithEventID(t *testing.T) {
	stream := "id: 42\nevent: test\ndata: payload\n\n"
	reader := NewReader(nopCloser{strings.NewReader(stream)})

	evt, err := reader.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.ID != "42" {
		t.Errorf("expected ID '42', got %q", evt.ID)
	}
}

func TestReaderEmptyStream(t *testing.T) {
	reader := NewReader(nopCloser{strings.NewReader("")})

	_, err := reader.Next()
	if err != io.EOF {
		t.Errorf("expected EOF on empty stream, got: %v", err)
	}
}

func TestReaderDataWithColon(t *testing.T) {
	stream := "data: key: value\n\n"
	reader := NewReader(nopCloser{strings.NewReader(stream)})

	evt, err := reader.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Data != "key: value" {
		t.Errorf("expected 'key: value', got %q", evt.Data)
	}
}

func TestReaderPendingDataAtEOF(t *testing.T) {
	// Stream that ends without a trailing blank line.
	stream := "event: test\ndata: final"
	reader := NewReader(nopCloser{strings.NewReader(stream)})

	evt, err := reader.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "test" || evt.Data != "final" {
		t.Errorf("unexpected event: type=%q data=%q", evt.Type, evt.Data)
	}
}
