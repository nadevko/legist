package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// TrailingSlash normalizes trailing slashes for GET requests to swagger routes.
func TrailingSlash(basePath string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Method == http.MethodGet {
				path := c.Request().URL.Path
				// Redirect /api/swagger to /api/swagger/
				if path == basePath+"/swagger" {
					return c.Redirect(http.StatusMovedPermanently, path+"/")
				}
				// Redirect /api to /api/
				if path == basePath {
					return c.Redirect(http.StatusMovedPermanently, path+"/")
				}
			}
			return next(c)
		}
	}
}
