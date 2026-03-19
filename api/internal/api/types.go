package api

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/pagination"
)

// --- error types ---

type apiError struct {
	Type    string  `json:"type"`
	Code    string  `json:"code"`
	Message string  `json:"message"`
	Param   *string `json:"param,omitempty"`
}

type apiErrorResponse struct {
	Object string   `json:"object"` // "error"
	Error  apiError `json:"error"`
}

const (
	errTypeInvalidRequest = "invalid_request_error"
	errTypeAuth           = "authentication_error"
	errTypeAuthz          = "authorization_error"
	errTypeNotFound       = "not_found_error"
	errTypeConflict       = "conflict_error"
	errTypeServer         = "api_error"
)

type httpError struct {
	code    string
	message string
	param   *string
}

func errorf(status int, code, message string, param ...string) error {
	e := &httpError{code: code, message: message}
	if len(param) > 0 {
		e.param = &param[0]
	}
	return echo.NewHTTPError(status, e)
}

func errorHandler(err error, c echo.Context) {
	code := http.StatusInternalServerError
	resp := apiErrorResponse{
		Object: "error",
		Error:  apiError{Type: errTypeServer, Code: "server_error", Message: "internal error"},
	}
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		errType := httpCodeToType(code)
		switch msg := he.Message.(type) {
		case *httpError:
			resp.Error = apiError{Type: errType, Code: msg.code, Message: msg.message, Param: msg.param}
		case *auth.AuthError:
			resp.Error = apiError{Type: errTypeAuth, Code: msg.Code, Message: msg.Message}
		case string:
			resp.Error = apiError{Type: errType, Code: httpCodeToCode(code), Message: msg}
		default:
			resp.Error = apiError{Type: errType, Code: httpCodeToCode(code), Message: "internal error"}
		}
	}
	c.JSON(code, resp)
}

func httpCodeToType(code int) string {
	switch code {
	case http.StatusBadRequest:
		return errTypeInvalidRequest
	case http.StatusUnauthorized:
		return errTypeAuth
	case http.StatusForbidden:
		return errTypeAuthz
	case http.StatusNotFound:
		return errTypeNotFound
	case http.StatusConflict:
		return errTypeConflict
	default:
		return errTypeServer
	}
}

func httpCodeToCode(code int) string {
	switch code {
	case http.StatusBadRequest:
		return "invalid_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	default:
		return "server_error"
	}
}

// --- deleted response ---

type deletedResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}

func deleted(id, object string) deletedResponse {
	return deletedResponse{ID: id, Object: object, Deleted: true}
}

// --- list response ---

type listResponse[T any] struct {
	Object     string `json:"object"` // "list"
	Data       []T    `json:"data"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// --- pagination ---

type listParams struct {
	Limit         int    `query:"limit"`
	StartingAfter string `query:"starting_after"`
	EndingBefore  string `query:"ending_before"`
}

func (p *listParams) normalize() {
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}
}

func (p *listParams) toStore() pagination.Params {
	return pagination.Params{
		Limit:         p.Limit,
		StartingAfter: p.StartingAfter,
		EndingBefore:  p.EndingBefore,
	}
}

// --- resource responses ---

type userResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"` // "user"
	Email   string `json:"email"`
	Created int64  `json:"created"`
}

type sessionResponse struct {
	ID        string `json:"id"`
	Object    string `json:"object"` // "session"
	UserID    string `json:"user_id"`
	ExpiresAt int64  `json:"expires_at"`
	Created   int64  `json:"created"`
}

type fileResponse struct {
	ID         string  `json:"id"`
	Object     string  `json:"object"` // "file"
	DocumentID *string `json:"document_id,omitempty"`
	UserID     *string `json:"user_id,omitempty"`
	Name       string  `json:"name"`
	MimeType   string  `json:"mime_type"`
	Size       int64   `json:"size"`
	Status     string  `json:"status"`
	// Expression-level — may be null if not yet extracted
	VersionDate   *string `json:"version_date,omitempty"`
	VersionNumber *string `json:"version_number,omitempty"`
	VersionLabel  *string `json:"version_label,omitempty"`
	Language      *string `json:"language,omitempty"`
	Created       int64   `json:"created"`
}

type tokenResponse struct {
	Object       string `json:"object"` // "token"
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

var allowedMIME = map[string]bool{
	"application/pdf": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
}

func toUnix(t time.Time) int64 { return t.Unix() }
