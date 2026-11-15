package api

import (
	"io"
	"net/http"
	"strings"
)

// sseWriter writes a Server-Sent Events response. It is not safe for
// concurrent use; one goroutine owns the stream.
type sseWriter struct {
	w  io.Writer
	rc *http.ResponseController
}

// startSSE sends the headers of an event stream and flushes them, so the
// client sees the stream open before the first event.
func startSSE(w http.ResponseWriter) (*sseWriter, error) {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	// Reverse proxies (nginx, ingress-nginx) must not buffer the stream.
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	s := &sseWriter{w: w, rc: http.NewResponseController(w)}
	return s, s.flush()
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
	_, err := io.WriteString(s.w, b.String())
	return err
}

// comment writes a comment line, which clients ignore; used as heartbeat.
func (s *sseWriter) comment(text string) error {
	_, err := io.WriteString(s.w, ": "+text+"\n\n")
	return err
}

func (s *sseWriter) flush() error { return s.rc.Flush() }
