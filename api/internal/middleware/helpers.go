package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
)

// bufferedWriter buffers response body and intercepts status code.
// Used in expand and idempotency middleware.
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

// errorResp creates a JSON response for error messages in standard API format.
func errorResp(errType, code, message string) json.RawMessage {
	resp := map[string]interface{}{
		"object": "error",
		"error": map[string]interface{}{
			"type":    errType,
			"code":    code,
			"message": message,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}
