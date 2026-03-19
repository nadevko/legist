package api

import (
	"bytes"
	"net/http"
)

// bufferedWriter буферизует тело ответа и перехватывает статус код.
// Используется в expand и idempotency middleware.
type bufferedWriter struct {
	http.ResponseWriter
	buf    *bytes.Buffer
	status int
}

func newBufferedWriter(rw http.ResponseWriter) *bufferedWriter {
	return &bufferedWriter{
		ResponseWriter: rw,
		buf:            &bytes.Buffer{},
		status:         http.StatusOK,
	}
}

func (w *bufferedWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

func (w *bufferedWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
