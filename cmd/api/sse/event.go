package sse

import (
	"encoding/json"
	"fmt"
	"io"
)

// Event is a single server-sent event.
type Event struct {
	Type    string
	Payload any
}

// Write serialises the event in text/event-stream wire format and flushes it
// to w.
func (e Event) Write(w io.Writer) error {
	data, err := json.Marshal(e.Payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, data)
	return err
}
