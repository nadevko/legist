package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/nadevko/legist/internal/config"
)

// CORS creates a CORS middleware based on dev/prod mode.
// Dev: allow all origins. Prod: allow only PublicHost.
func CORS(cfg *config.Config) echo.MiddlewareFunc {
	corsConfig := middleware.CORSConfig{
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			"Content-Type",
			"Authorization",
			"Idempotency-Key",
			"Legist-Version",
		},
	}
	if cfg.Dev {
		corsConfig.AllowOrigins = []string{"*"}
	} else {
		corsConfig.AllowOrigins = []string{cfg.PublicHost}
	}
	return middleware.CORSWithConfig(corsConfig)
}
