package middleware

import (
	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
)

// Version adds the Legist-Version header to every response.
// If the request carries a mismatched Legist-Version header, a warning is added.
func Version() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Legist-Version", auth.Version)
			if v := c.Request().Header.Get("Legist-Version"); v != "" && v != auth.Version {
				c.Response().Header().Set("Legist-Version-Warning",
					"requested version "+v+" is not supported, using "+auth.Version)
			}
			return next(c)
		}
	}
}
