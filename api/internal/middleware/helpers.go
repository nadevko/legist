package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
)

// bufferedWriter buffers the response body and intercepts the status code.
// Used by expand and idempotency middleware to post-process responses.
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

// parseExpand parses expand[] query parameters into a set.
func parseExpand(c echo.Context) map[string]bool {
	result := map[string]bool{}
	for _, v := range c.QueryParams()["expand[]"] {
		result[v] = true
	}
	return result
}

// errorResp builds a standard API error JSON blob for use in middleware
// before the full Echo error handler is available.
func errorResp(errType, code, message string) json.RawMessage {
	resp := map[string]any{
		"object": "error",
		"error": map[string]any{
			"type":    errType,
			"code":    code,
			"message": message,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}
