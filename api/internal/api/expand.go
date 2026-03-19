package api

import (
	"strings"
)

// expandJSON recursively expands _id fields in the JSON data tree.
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
		if loaded := s.LoadResource(resource, id); loaded != nil {
			delete(obj, key)
			obj[resource] = loaded
		}
	}
	for key, val := range obj {
		obj[key] = s.expandJSON(val, expand)
	}
	return obj
}
