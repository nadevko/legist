package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

func TrailingSlash(basePath string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Method != http.MethodGet {
				return next(c)
			}

			req := c.Request()
			path := req.URL.Path

			base := strings.TrimRight(basePath, "/")

			if path == base || path == base+"/swagger" {
				query := ""
				if req.URL.RawQuery != "" {
					query = "?" + req.URL.RawQuery
				}

				return c.Redirect(
					http.StatusTemporaryRedirect,
					path+"/"+query,
				)
			}

			return next(c)
		}
	}
}
