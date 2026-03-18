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
)

type Server struct {
	e        *echo.Echo
	cfg      *config.Config
	users    *store.UserStore
	sessions *store.SessionStore
	resets   *store.PasswordResetStore
	files    *store.FileStore
	jobs     *store.JobStore
	broker   *sse.Broker
}

func NewServer(cfg *config.Config, db *sqlx.DB) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		code := http.StatusInternalServerError
		msg := "internal error"
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			if s, ok := he.Message.(string); ok {
				msg = s
			}
		}
		c.JSON(code, echo.Map{"message": msg})
	}

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())

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
		}))
	}

	s := &Server{
		e:        e,
		cfg:      cfg,
		users:    store.NewUserStore(db),
		sessions: store.NewSessionStore(db),
		resets:   store.NewPasswordResetStore(db),
		files:    store.NewFileStore(db),
		jobs:     store.NewJobStore(db),
		broker:   sse.NewBroker(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.e.GET("/swagger", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	s.e.GET("/swagger/*", echoSwagger.EchoWrapHandler(func(c *echoSwagger.Config) {
		c.URLs = []string{"v1-alpha.json"}
	}))
	s.e.GET("/swagger/v1-alpha.json", func(c echo.Context) error {
		return c.File("docs/v1-alpha.json")
	})
	s.e.GET("/health", s.handleHealth)

	// public
	s.e.POST("/users", s.handleRegister)
	s.e.POST("/sessions", s.handleLogin)
	s.e.POST("/tokens/refresh", s.handleRefresh)
	s.e.POST("/tokens/password-reset", s.handleRequestPasswordReset)
	s.e.PATCH("/tokens/password-reset", s.handleChangePassword)

	// protected
	p := s.e.Group("", auth.Middleware(s.cfg.JWTSecret))
	p.DELETE("/sessions", s.handleLogout)
	p.GET("/me", s.handleMe)
	p.DELETE("/me", s.handleDeleteMe)

	p.POST("/files", s.handleUploadFiles)
	p.GET("/files", s.handleListFiles)
	p.GET("/files/:id", s.handleGetFile)
	p.GET("/files/:id.sse", s.handleFileSSE)
	p.DELETE("/files/:id", s.handleDeleteFile)

	p.GET("/jobs/:id", s.handleGetJob)
	p.GET("/jobs/:id.sse", s.handleJobSSE)
}

func (s *Server) Start() error {
	return s.e.Start(s.cfg.Addr)
}
