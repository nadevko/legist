package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

func (s *Server) expandMiddleware() echo.MiddlewareFunc {
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

			expanded := s.expandJSON(data, expand)
			c.Response().WriteHeader(rw.status)
			return json.NewEncoder(c.Response().Writer).Encode(expanded)
		}
	}
}

func (s *Server) expandJSON(data any, expand map[string]bool) any {
	switch v := data.(type) {
	case map[string]any:
		return s.expandObject(v, expand)
	case []any:
		for i, item := range v {
			v[i] = s.expandJSON(item, expand)
		}
		return v
	default:
		return data
	}
}

func (s *Server) expandObject(obj map[string]any, expand map[string]bool) map[string]any {
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
		if loaded := s.loadResource(resource, id); loaded != nil {
			delete(obj, key)
			obj[resource] = loaded
		}
	}
	for key, val := range obj {
		obj[key] = s.expandJSON(val, expand)
	}
	return obj
}

func (s *Server) loadResource(resource, id string) any {
	switch resource {
	case "user":
		u, err := s.users.GetByID(id)
		if err != nil {
			return nil
		}
		return toUserResponse(*u)
	case "file":
		f, err := s.files.GetByID(id)
		if err != nil {
			return nil
		}
		return toFileResponse(*f)
	default:
		return nil
	}
}

// expandResponseWriter удалён — используем bufferedWriter из writer.go
var _ = http.StatusOK // keep net/http import used via bufferedWriter
