package api

import (
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/config"
	mw "github.com/nadevko/legist/internal/middleware"
	"github.com/nadevko/legist/internal/sse"
	"github.com/nadevko/legist/internal/store"
	"github.com/nadevko/legist/internal/webhook"
)

// Server encapsulates all HTTP handlers and dependencies.
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

// NewServer creates and configures a new HTTP server with all middleware and routes.
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
	e.Use(mw.Version())
	e.Use(mw.TrailingSlash(cfg.BasePath))
	e.Use(mw.Expand(s))
	e.Use(mw.Idempotency(s.idempotency))
	e.Use(mw.CORS(cfg))

	s.registerRoutes()
	return s
}

// registerRoutes sets up all API routes.
func (s *Server) registerRoutes() {
	e := s.e

	e.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, s.cfg.BasePath+"/swagger/")
	})

	// API routes with base path
	api := e.Group(s.cfg.BasePath)

	// Redirect /api/ to /api/swagger/
	api.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, s.cfg.BasePath+"/swagger/")
	})

	// Swagger routes (public) - wildcard catches /api/swagger/ and everything under it
	api.GET("/swagger/*", echoSwagger.EchoWrapHandler(func(c *echoSwagger.Config) {
		c.URLs = []string{"v1-alpha.json"}
	}))
	api.GET("/swagger/v1-alpha.json", func(c echo.Context) error {
		return c.File("docs/v1-alpha.json")
	})
	api.GET("/health", s.handleHealth)

	// public — auth
	api.POST("/users", s.handleRegister)
	api.POST("/sessions", s.handleLogin)
	api.POST("/tokens/refresh", s.handleRefresh)
	api.POST("/tokens/password-reset", s.handleRequestPasswordReset)
	api.PATCH("/tokens/password-reset", s.handleChangePassword)

	// protected
	p := api.Group("", auth.Middleware(s.cfg.JWTSecret))

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

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	return s.e.Start(s.cfg.Addr)
}
