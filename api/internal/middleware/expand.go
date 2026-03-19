package middleware

import (
	"encoding/json"
	"strings"

	"github.com/labstack/echo/v4"
)

// ExpandLoader defines methods needed for expand middleware to load resources.
type ExpandLoader interface {
	LoadResource(resource, id string) any
}

// Expand creates a middleware that expands related resources based on query parameter.
// The expander must implement ExpandLoader interface.
func Expand(expander ExpandLoader) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			expand := parseExpand(c)
			if len(expand) == 0 {
				return next(c)
			}

			if c.Request().Header.Get("Accept") == "text/event-stream" {
				return next(c)
			}

			rw := newBufferedWriter(c.Response().Writer)
			c.Response().Writer = rw

			if err := next(c); err != nil {
				c.Response().Writer = rw.ResponseWriter
				return err
			}

			c.Response().Writer = rw.ResponseWriter

			ct := rw.ResponseWriter.Header().Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				c.Response().Writer.Write(rw.buf.Bytes())
				return nil
			}

			var data any
			if err := json.Unmarshal(rw.buf.Bytes(), &data); err != nil {
				c.Response().Writer.Write(rw.buf.Bytes())
				return nil
			}

			expanded := expandJSON(expander, data, expand)
			c.Response().WriteHeader(rw.status)
			return json.NewEncoder(c.Response().Writer).Encode(expanded)
		}
	}
}

func expandJSON(expander ExpandLoader, data any, expand map[string]bool) any {
	switch v := data.(type) {
	case map[string]any:
		return expandObject(expander, v, expand)
	case []any:
		for i, item := range v {
			v[i] = expandJSON(expander, item, expand)
		}
		return v
	default:
		return data
	}
}

func expandObject(expander ExpandLoader, obj map[string]any, expand map[string]bool) map[string]any {
	for key, val := range obj {
		if !strings.HasSuffix(key, "_id") {
			continue
		}
		resource := strings.TrimSuffix(key, "_id")
		if !expand[resource] {
			continue
		}
		id, ok := val.(string)
		if !ok {
			continue
		}
		if loaded := expander.LoadResource(resource, id); loaded != nil {
			delete(obj, key)
			obj[resource] = loaded
		}
	}
	for key, val := range obj {
		obj[key] = expandJSON(expander, val, expand)
	}
	return obj
}
