package rue

import (
	"bufio"
	"net"
	"net/http"
)

// ResponseWriter extends http.ResponseWriter with status tracking, size
// accounting, and protocol upgrade helpers (hijack, flush, push).
type ResponseWriter interface {
	http.ResponseWriter
	http.Hijacker
	http.Flusher

	// Status reports the status code set via WriteHeader
	Status() int
	// Size reports the total bytes written to the body
	Size() int
	// Written reports whether headers have been flushed to the network
	Written() bool
	// WriteString is a convenience method equivalent to Write([]byte(s))
	WriteString(s string) (int, error)
	// WriteHeaderNow flushes the status line and headers immediately
	WriteHeaderNow()
}

// responseWriter tracks response state for a single request
type responseWriter struct {
	http.ResponseWriter
	status  int
	size    int
	written bool
}

// WriteHeader records the status code without flushing
func (w *responseWriter) WriteHeader(code int) {
	if w.written {
		return
	}
	w.status = code
}

// WriteHeaderNow flushes status and headers to the underlying connection
func (w *responseWriter) WriteHeaderNow() {
	if !w.written {
		w.written = true
		w.ResponseWriter.WriteHeader(w.status)
	}
}

// Write writes data to the response
func (w *responseWriter) Write(data []byte) (int, error) {
	w.WriteHeaderNow()
	n, err := w.ResponseWriter.Write(data)
	w.size += n
	return n, err
}

// WriteString writes a string to the response
func (w *responseWriter) WriteString(s string) (int, error) {
	w.WriteHeaderNow()
	n, err := w.ResponseWriter.Write([]byte(s))
	w.size += n
	return n, err
}

// Status returns the HTTP response status code
func (w *responseWriter) Status() int {
	return w.status
}

// Size returns the number of bytes written
func (w *responseWriter) Size() int {
	return w.size
}

// Written returns whether the response has been written
func (w *responseWriter) Written() bool {
	return w.written
}

// Hijack implements the http.Hijacker interface
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Flush implements the http.Flusher interface
func (w *responseWriter) Flush() {
	w.WriteHeaderNow()
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Pusher returns the http.Pusher for HTTP/2 server push
func (w *responseWriter) Pusher() http.Pusher {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher
	}
	return nil
}
