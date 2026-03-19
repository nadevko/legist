package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

func joinPath(basePath, suffix string) string {
	base := strings.Trim(basePath, "/")
	suf := strings.Trim(suffix, "/")
	switch {
	case base == "" && suf == "":
		return "/"
	case base == "":
		return "/" + suf
	case suf == "":
		return "/" + base
	default:
		return "/" + base + "/" + suf
	}
}

func TrailingSlash(basePath string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Method != http.MethodGet {
				return next(c)
			}

			req := c.Request()
			path := req.URL.Path

			apiRoot := joinPath(basePath, "")
			swaggerRoot := joinPath(basePath, "swagger")
			swaggerIndex := joinPath(basePath, "swagger/index.html")

			redirect := func(status int, target string) error {
				if req.URL.RawQuery != "" {
					target += "?" + req.URL.RawQuery
				}
				return c.Redirect(status, target)
			}

			// Canonical Swagger UI entrypoints.
			if path == apiRoot || path == apiRoot+"/" ||
				path == swaggerRoot || path == swaggerRoot+"/" {
				return redirect(http.StatusMovedPermanently, swaggerIndex)
			}

			// Do not normalize Swagger asset paths.
			if path == swaggerRoot || strings.HasPrefix(path, swaggerRoot+"/") {
				return next(c)
			}

			// Stripe-style canonical URLs: strip trailing slash everywhere else.
			if len(path) > 1 && strings.HasSuffix(path, "/") {
				return redirect(http.StatusTemporaryRedirect, strings.TrimSuffix(path, "/"))
			}

			return next(c)
		}
	}
}
