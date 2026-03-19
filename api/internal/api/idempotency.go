package api

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/store"
)

func (s *Server) idempotencyMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			method := c.Request().Method
			if method != http.MethodPost &&
				method != http.MethodPatch &&
				method != http.MethodDelete {
				return next(c)
			}

			key := c.Request().Header.Get("Idempotency-Key")
			if key == "" {
				if method == http.MethodPost {
					return errorf(http.StatusBadRequest, "missing_idempotency_key",
						"Idempotency-Key header is required for POST requests")
				}
				return next(c)
			}

			userID := auth.UserID(c)
			if userID == "" {
				userID = c.RealIP()
			}

			path := c.Request().URL.Path

			// проверяем существующий ключ
			existing, err := s.idempotency.Get(key, userID)
			if err == nil {
				if existing.Method != method || existing.Path != path {
					return errorf(http.StatusUnprocessableEntity, "idempotency_key_reuse",
						"idempotency key already used for a different request")
				}
				if existing.Response != "" {
					// завершённый запрос — возвращаем кэш
					c.Response().Header().Set("Idempotency-Key", key)
					c.Response().Header().Set("Idempotent-Replayed", "true")
					return c.JSONBlob(existing.Status, []byte(existing.Response))
				}
				// response пустой — запрос ещё выполняется
				return errorf(http.StatusConflict, "idempotency_key_in_use",
					"a request with this idempotency key is already in progress")
			}

			// резервируем ключ
			if err = s.idempotency.Lock(key, userID, method, path); err != nil {
				if errors.Is(err, store.ErrConflict) {
					return errorf(http.StatusConflict, "idempotency_key_in_use",
						"a request with this idempotency key is already in progress")
				}
				return errorf(http.StatusInternalServerError, "server_error", "internal error")
			}

			// перехватываем ответ
			rw := newBufferedWriter(c.Response().Writer)
			c.Response().Writer = rw

			if err := next(c); err != nil {
				c.Response().Writer = rw.ResponseWriter
				return err
			}

			// сохраняем результат
			s.idempotency.Set(&store.IdempotencyKey{
				Key:      key,
				UserID:   userID,
				Method:   method,
				Path:     path,
				Status:   rw.status,
				Response: rw.buf.String(),
			})

			c.Response().Header().Set("Idempotency-Key", key)
			c.Response().Writer = rw.ResponseWriter
			c.Response().Writer.Write(rw.buf.Bytes())
			return nil
		}
	}
}
