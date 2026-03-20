package middleware

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/store"
)

// Idempotency creates a middleware that handles idempotent requests.
// In dev we disable this middleware completely to avoid requiring `Idempotency-Key`.
// POST requests require Idempotency-Key. PATCH and DELETE are optional.
func Idempotency(idempotencyStore *store.IdempotencyStore, dev bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			method := c.Request().Method
			if method != http.MethodPost &&
				method != http.MethodPatch &&
				method != http.MethodDelete {
				return next(c)
			}

			if dev {
				return next(c)
			}

			key := c.Request().Header.Get("Idempotency-Key")
			if key == "" {
				if method == http.MethodPost {
					return echo.NewHTTPError(http.StatusBadRequest,
						errorResp("invalid_request_error", "missing_idempotency_key",
							"Idempotency-Key header is required for POST requests"))
				}
				return next(c)
			}

			userID := auth.UserID(c)
			if userID == "" {
				userID = c.RealIP()
			}
			path := c.Request().URL.Path

			// Check for existing key.
			existing, err := idempotencyStore.Get(key, userID)
			if err == nil {
				if existing.Method != method || existing.Path != path {
					return echo.NewHTTPError(http.StatusUnprocessableEntity,
						errorResp("conflict_error", "idempotency_key_reuse",
							"idempotency key already used for a different request"))
				}
				if existing.Response != "" {
					c.Response().Header().Set("Idempotency-Key", key)
					c.Response().Header().Set("Idempotent-Replayed", "true")
					return c.JSONBlob(existing.Status, []byte(existing.Response))
				}
				return echo.NewHTTPError(http.StatusConflict,
					errorResp("conflict_error", "idempotency_key_in_use",
						"a request with this idempotency key is already in progress"))
			}

			// Reserve the key.
			if err = idempotencyStore.Lock(key, userID, method, path); err != nil {
				if errors.Is(err, store.ErrIdempotencyConflict) {
					return echo.NewHTTPError(http.StatusConflict,
						errorResp("conflict_error", "idempotency_key_in_use",
							"a request with this idempotency key is already in progress"))
				}
				return echo.NewHTTPError(http.StatusInternalServerError,
					errorResp("api_error", "server_error", "internal error"))
			}

			// Intercept the response.
			rw := newBufferedWriter(c.Response().Writer)
			c.Response().Writer = rw

			if err = next(c); err != nil {
				c.Response().Writer = rw.ResponseWriter
				return err
			}

			// Persist result.
			if err = idempotencyStore.Set(&store.IdempotencyKey{
				Key:      key,
				UserID:   userID,
				Method:   method,
				Path:     path,
				Status:   rw.status,
				Response: rw.buf.String(),
			}); err != nil {
				// Log but don't fail — response already written.
				_ = err
			}

			c.Response().Header().Set("Idempotency-Key", key)
			c.Response().Writer = rw.ResponseWriter
			c.Response().Writer.Write(rw.buf.Bytes())
			return nil
		}
	}
}
