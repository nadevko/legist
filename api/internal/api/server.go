package api

import (
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
	documents   *store.DocumentStore
	diffs       *store.DiffStore
	idempotency *store.IdempotencyStore
	regexRules  *store.RegexRulesStore
	webhooks    *store.WebhookStore
	dispatcher  *webhook.Dispatcher
	broker      *sse.Broker
}

// NewServer creates and configures the HTTP server.
func NewServer(cfg *config.Config, db *sqlx.DB) *Server {
	webhookStore := store.NewWebhookStore(db)
	s := &Server{
		e:           echo.New(),
		cfg:         cfg,
		users:       store.NewUserStore(db),
		sessions:    store.NewSessionStore(db),
		resets:      store.NewPasswordResetStore(db),
		files:       store.NewFileStore(db),
		documents:   store.NewDocumentStore(db),
		diffs:       store.NewDiffStore(db),
		idempotency: store.NewIdempotencyStore(db),
		regexRules:  store.NewRegexRulesStore(db),
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
	e.Use(mw.Idempotency(s.idempotency, cfg.Dev))
	e.Use(mw.CORS(cfg))

	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	e := s.e

	api := e.Group(s.cfg.BasePath)

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

	// users
	p.GET("/me", s.handleGetUser)
	p.PATCH("/me", s.handleUpdateUser)
	p.DELETE("/me", s.handleDeleteUser)
	p.GET("/users/:id", s.handleGetUser)
	p.PATCH("/users/:id", s.handleUpdateUser)
	p.DELETE("/users/:id", s.handleDeleteUser)

	// sessions
	p.GET("/sessions", s.handleListSessions)
	p.DELETE("/sessions/:id", s.handleLogout)

	// files
	// POST /files                       — creates new Document automatically
	// POST /files (document_id present) — alias → POST /documents/:id/files
	// GET  /files (document_id present) — alias → GET  /documents/:id/files
	p.GET("/files", s.handleListFiles)
	p.GET("/files/:id", s.handleGetFile)
	p.POST("/files", s.handleUploadFile)
	p.PATCH("/files/:id", s.handlePatchFile)
	p.DELETE("/files/:id", s.handleDeleteFile)

	// documents — canonical endpoints; /files aliases forward here
	p.POST("/documents", s.handleCreateDocument)
	p.GET("/documents", s.handleListDocuments)
	p.GET("/documents/:id", s.handleGetDocument)
	p.PATCH("/documents/:id", s.handleUpdateDocument)
	p.DELETE("/documents/:id", s.handleDeleteDocument)
	p.GET("/documents/:id/files", s.handleListDocumentFiles)   // canonical list versions
	p.POST("/documents/:id/files", s.handleUploadDocumentFile) // canonical add version

	// diffs
	p.POST("/diffs", s.handleCreateDiff)
	p.GET("/diffs", s.handleListDiffs)
	p.GET("/diffs/:id", s.handleGetDiff)

	// reports (RAG annotated diff -> optional docx)
	p.GET("/reports/:diff_id", s.handleGetReport)

	// regex rules (admin)
	p.GET("/regex/weights", s.handleListWeightRegexRules)
	p.PUT("/regex/weights", s.handleReplaceWeightRegexRules)
	p.POST("/regex/weights", s.handleCreateWeightRegexRule)
	p.DELETE("/regex/weights", s.handleResetWeightRegexRules)
	p.PATCH("/regex/weights/:id", s.handleUpdateWeightRegexRule)
	p.DELETE("/regex/weights/:id", s.handleDeleteWeightRegexRule)

	p.GET("/regex/omits", s.handleListOmitRegexRules)
	p.PUT("/regex/omits", s.handleReplaceOmitRegexRules)
	p.POST("/regex/omits", s.handleCreateOmitRegexRule)
	p.DELETE("/regex/omits", s.handleResetOmitRegexRules)
	p.PATCH("/regex/omits/:id", s.handleUpdateOmitRegexRule)
	p.DELETE("/regex/omits/:id", s.handleDeleteOmitRegexRule)

	// webhooks
	p.POST("/webhooks", s.handleCreateWebhook)
	p.GET("/webhooks", s.handleListWebhooks)
	p.GET("/webhooks/:id", s.handleGetWebhook)
	p.GET("/webhooks/:id/events", s.handleListWebhookEvents)
	p.PATCH("/webhooks/:id", s.handleUpdateWebhook)
	p.DELETE("/webhooks/:id", s.handleDeleteWebhook)

	// chat
	p.POST("/chat", s.handleChat)
}

// LoadResource implements middleware.ExpandLoader.
func (s *Server) LoadResource(resource, id string) any {
	switch resource {
	case "user":
		u, err := s.users.GetByID(id)
		if err != nil {
			return nil
		}
		return toUserResponse(*u)
	case "file":
		f, err := s.files.GetByID(id)
		if err != nil {
			return nil
		}
		return toFileResponse(*f)
	case "document":
		d, err := s.documents.GetByID(id)
		if err != nil {
			return nil
		}
		return toDocumentResponse(*d)
	case "diff":
		d, err := s.diffs.GetByID(id)
		if err != nil {
			return nil
		}
		return toDiffResponse(*d)
	case "left_file", "right_file":
		return s.LoadResource("file", id)
	default:
		return nil
	}
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	return s.e.Start(s.cfg.Addr)
}
