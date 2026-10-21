package controller

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const defaultRelayResponseBufferLimit = 64 << 20

var errRelayResponseBufferTooLarge = errors.New("relay response exceeds settlement buffer limit")

type relayResponseBuffer struct {
	underlying    gin.ResponseWriter
	header        http.Header
	initialHeader http.Header
	body          bytes.Buffer
	status        int
	written       bool
	passthrough   bool
	discarded     bool
	limit         int
}

var _ gin.ResponseWriter = (*relayResponseBuffer)(nil)

func newRelayResponseBuffer(underlying gin.ResponseWriter, limit int) *relayResponseBuffer {
	if limit <= 0 {
		limit = defaultRelayResponseBufferLimit
	}
	initial := cloneHTTPHeader(underlying.Header())
	return &relayResponseBuffer{
		underlying:    underlying,
		header:        cloneHTTPHeader(initial),
		initialHeader: initial,
		status:        http.StatusOK,
		limit:         limit,
	}
}

func cloneHTTPHeader(source http.Header) http.Header {
	cloned := make(http.Header, len(source))
	for key, values := range source {
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}

func replaceHTTPHeader(target, source http.Header) {
	for key := range target {
		delete(target, key)
	}
	for key, values := range source {
		target[key] = append([]string(nil), values...)
	}
}

func (w *relayResponseBuffer) Header() http.Header { return w.header }

func (w *relayResponseBuffer) WriteHeader(code int) {
	if w.discarded || w.written || code <= 0 {
		return
	}
	w.status = code
	w.written = true
	if w.isEventStream() {
		_ = w.activatePassthrough()
	}
}

func (w *relayResponseBuffer) WriteHeaderNow() {
	if !w.written {
		w.WriteHeader(w.status)
	}
	if w.isEventStream() {
		_ = w.activatePassthrough()
	}
}

func (w *relayResponseBuffer) Write(data []byte) (int, error) {
	if w.discarded {
		return 0, errors.New("relay response buffer was discarded")
	}
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	if w.passthrough || w.isEventStream() {
		if err := w.activatePassthrough(); err != nil {
			return 0, err
		}
		return w.underlying.Write(data)
	}
	if len(data) > w.limit-w.body.Len() {
		return 0, errRelayResponseBufferTooLarge
	}
	return w.body.Write(data)
}

func (w *relayResponseBuffer) WriteString(data string) (int, error) {
	return w.Write([]byte(data))
}

func (w *relayResponseBuffer) Status() int { return w.status }

func (w *relayResponseBuffer) Size() int {
	if !w.written {
		return -1
	}
	if w.passthrough {
		return w.underlying.Size()
	}
	return w.body.Len()
}

func (w *relayResponseBuffer) Written() bool { return w.written }

func (w *relayResponseBuffer) Pusher() http.Pusher { return w.underlying.Pusher() }

func (w *relayResponseBuffer) CloseNotify() <-chan bool { return w.underlying.CloseNotify() }

func (w *relayResponseBuffer) Flush() {
	if w.discarded {
		return
	}
	_ = w.activatePassthrough()
	w.underlying.Flush()
}

func (w *relayResponseBuffer) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if err := w.activatePassthrough(); err != nil {
		return nil, nil, err
	}
	return w.underlying.Hijack()
}

func (w *relayResponseBuffer) Unwrap() http.ResponseWriter { return w.underlying }

func (w *relayResponseBuffer) IsPassthrough() bool { return w.passthrough }

// ResponseForwarded reports whether any buffered response state has reached
// the real client. Billing uses this instead of treating every declared stream
// as delivered.
func (w *relayResponseBuffer) ResponseForwarded() bool { return w.passthrough }

func (w *relayResponseBuffer) isEventStream() bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(w.header.Get("Content-Type"))), "text/event-stream")
}

func (w *relayResponseBuffer) activatePassthrough() error {
	if w.passthrough {
		return nil
	}
	if w.discarded {
		return errors.New("relay response buffer was discarded")
	}
	replaceHTTPHeader(w.underlying.Header(), w.header)
	w.underlying.WriteHeader(w.status)
	if w.body.Len() > 0 {
		if _, err := io.Copy(w.underlying, &w.body); err != nil {
			return err
		}
	}
	w.passthrough = true
	return nil
}

func (w *relayResponseBuffer) Commit() error {
	if w.passthrough {
		return nil
	}
	return w.activatePassthrough()
}

func (w *relayResponseBuffer) Discard() {
	if w.passthrough {
		return
	}
	w.discarded = true
	w.body.Reset()
	replaceHTTPHeader(w.underlying.Header(), w.initialHeader)
}
