package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"kubebench/internal/reason"
)

// sseWriter writes a Server-Sent Events response. It is not safe for
// concurrent use; one goroutine owns the stream.
type sseWriter struct {
	w  io.Writer
	rc *http.ResponseController
	// writeWait bounds every write and flush: a client that stops reading
	// fails the stream rather than blocking it forever (the server has no
	// WriteTimeout, which would end every stream).
	writeWait time.Duration
}

// startSSE sends the headers of an event stream and flushes them, so the
// client sees the stream open before the first event.
func startSSE(w http.ResponseWriter, writeWait time.Duration) (*sseWriter, error) {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	// Reverse proxies (nginx, ingress-nginx) must not buffer the stream.
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	s := &sseWriter{w: w, rc: http.NewResponseController(w), writeWait: writeWait}
	return s, s.flush()
}

// deadline gives the next write writeWait to complete. Writers without
// deadlines (test recorders) are written to without one.
func (s *sseWriter) deadline() error {
	if err := s.rc.SetWriteDeadline(time.Now().Add(s.writeWait)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	return nil
}

// event writes one event of type name ("" for the default "message"
// type). Every line of data becomes a "data:" line; carriage returns are
// dropped because SSE treats them as line breaks.
func (s *sseWriter) event(name, data string) error {
	var b strings.Builder
	if name != "" {
		b.WriteString("event: ")
		b.WriteString(name)
		b.WriteByte('\n')
	}
	for line := range strings.SplitSeq(strings.ReplaceAll(data, "\r", ""), "\n") {
		b.WriteString("data: ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	if err := s.deadline(); err != nil {
		return err
	}
	_, err := io.WriteString(s.w, b.String())
	return err
}

// explain writes the event name with why as its data: a reason.Message
// ({"error", "reason", "params"}), which the UI shows in its language,
// and flushes it. It ends log streams ("error") and watch streams ("end").
func (s *sseWriter) explain(name string, why error) error {
	data, err := json.Marshal(reason.MessageOf(why))
	if err != nil {
		return err
	}
	if err := s.event(name, string(data)); err != nil {
		return err
	}
	return s.flush()
}

// comment writes a comment line, which clients ignore; used as heartbeat.
func (s *sseWriter) comment(text string) error {
	if err := s.deadline(); err != nil {
		return err
	}
	_, err := io.WriteString(s.w, ": "+text+"\n\n")
	return err
}

// end clears the write deadline once the stream is over: net/http keeps
// the connection for further requests and does not reset it.
func (s *sseWriter) end() { _ = s.rc.SetWriteDeadline(time.Time{}) }

func (s *sseWriter) flush() error {
	if err := s.deadline(); err != nil {
		return err
	}
	return s.rc.Flush()
}
