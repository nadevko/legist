package api

import (
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/config"
	"github.com/nadevko/legist/internal/store"
)

type Server struct {
	e        *echo.Echo
	cfg      *config.Config
	users    *store.UserStore
	sessions *store.SessionStore
}

func NewServer(cfg *config.Config, db *sqlx.DB) *Server {
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())

	if cfg.Dev {
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{
				http.MethodGet,
				http.MethodPost,
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
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.e.GET("/swagger/v1-alpha.json", func(c echo.Context) error {
		return c.File("docs/v1-alpha.json")
	})
	s.e.GET("/swagger/*", echoSwagger.EchoWrapHandler(func(c *echoSwagger.Config) {
		c.URLs = []string{"v1-alpha.json"}
	}))
	s.e.GET("/health", s.handleHealth)

	s.e.POST("/users", s.handleRegister)
	s.e.POST("/sessions", s.handleLogin)
	s.e.POST("/tokens", s.handleRefresh)

	protected := s.e.Group("", auth.Middleware(s.cfg.JWTSecret))
	protected.DELETE("/sessions", s.handleLogout)
	protected.GET("/me", s.handleMe)
}

func (s *Server) Start() error {
	return s.e.Start(s.cfg.Addr)
}
