package auth

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type contextKey string

const UserIDKey contextKey = "userID"

func Middleware(secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing token")
			}
			claims, err := ParseAccessToken(strings.TrimPrefix(header, "Bearer "), secret)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}
			c.Set(string(UserIDKey), claims.UserID)
			return next(c)
		}
	}
}

// UserID достаёт userID из контекста Echo — хелпер для хендлеров.
func UserID(c echo.Context) string {
	v, _ := c.Get(string(UserIDKey)).(string)
	return v
}
