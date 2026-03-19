package api

import (
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/config"
	"github.com/nadevko/legist/internal/sse"
	"github.com/nadevko/legist/internal/store"
	"github.com/nadevko/legist/internal/webhook"
)

type Server struct {
	e           *echo.Echo
	cfg         *config.Config
	users       *store.UserStore
	sessions    *store.SessionStore
	resets      *store.PasswordResetStore
	files       *store.FileStore
	idempotency *store.IdempotencyStore
	webhooks    *store.WebhookStore
	dispatcher  *webhook.Dispatcher
	broker      *sse.Broker
}

func NewServer(cfg *config.Config, db *sqlx.DB) *Server {
	webhookStore := store.NewWebhookStore(db)
	s := &Server{
		e:           echo.New(),
		cfg:         cfg,
		users:       store.NewUserStore(db),
		sessions:    store.NewSessionStore(db),
		resets:      store.NewPasswordResetStore(db),
		files:       store.NewFileStore(db),
		idempotency: store.NewIdempotencyStore(db),
		webhooks:    webhookStore,
		dispatcher:  webhook.NewDispatcher(webhookStore),
		broker:      sse.NewBroker(),
	}

	e := s.e
	e.HideBanner = true
	e.HTTPErrorHandler = errorHandler

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestIDWithConfig(middleware.RequestIDConfig{
		RequestIDHandler: func(c echo.Context, id string) {
			c.Response().Header().Set("Request-Id", id)
		},
	}))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Legist-Version", auth.Version)
			if v := c.Request().Header.Get("Legist-Version"); v != "" && v != auth.Version {
				c.Response().Header().Set("Legist-Version-Warning",
					"requested version "+v+" is not supported, using "+auth.Version)
			}
			return next(c)
		}
	})
	e.Use(s.expandMiddleware())
	e.Use(s.idempotencyMiddleware())

	if cfg.Dev {
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{"*"},
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
		}))
	}

	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	e := s.e

	e.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	e.GET("/swagger", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	e.GET("/swagger/*", echoSwagger.EchoWrapHandler(func(c *echoSwagger.Config) {
		c.URLs = []string{"v1-alpha.json"}
	}))
	e.GET("/swagger/v1-alpha.json", func(c echo.Context) error {
		return c.File("docs/v1-alpha.json")
	})
	e.GET("/health", s.handleHealth)

	e.GET("/swagger", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	e.GET("/swagger/*", echoSwagger.EchoWrapHandler(func(c *echoSwagger.Config) {
		c.URLs = []string{"v1-alpha.json"}
	}))
	e.GET("/swagger/v1-alpha.json", func(c echo.Context) error {
		return c.File("docs/v1-alpha.json")
	})
	e.GET("/health", s.handleHealth)

	// public — auth
	e.POST("/users", s.handleRegister)
	e.POST("/sessions", s.handleLogin)
	e.POST("/tokens/refresh", s.handleRefresh)
	e.POST("/tokens/password-reset", s.handleRequestPasswordReset)
	e.PATCH("/tokens/password-reset", s.handleChangePassword)

	// protected
	p := e.Group("", auth.Middleware(s.cfg.JWTSecret))

	p.GET("/me", s.handleGetUser)
	p.PATCH("/me", s.handleUpdateUser)
	p.DELETE("/me", s.handleDeleteUser)
	p.GET("/users/:id", s.handleGetUser)
	p.PATCH("/users/:id", s.handleUpdateUser)
	p.DELETE("/users/:id", s.handleDeleteUser)

	p.GET("/sessions", s.handleListSessions)
	p.DELETE("/sessions/:id", s.handleLogout)

	p.GET("/files", s.handleListFiles)
	p.GET("/files/:id", s.handleGetFile)
	p.POST("/files", s.handleUploadFile)
	p.DELETE("/files/:id", s.handleDeleteFile)

	p.POST("/webhooks", s.handleCreateWebhook)
	p.GET("/webhooks", s.handleListWebhooks)
	p.GET("/webhooks/:id", s.handleGetWebhook)
	p.GET("/webhooks/:id/events", s.handleListWebhookEvents)
	p.PATCH("/webhooks/:id", s.handleUpdateWebhook)
	p.DELETE("/webhooks/:id", s.handleDeleteWebhook)

	p.POST("/chat", s.handleChat)
}

func (s *Server) Start() error {
	return s.e.Start(s.cfg.Addr)
}
