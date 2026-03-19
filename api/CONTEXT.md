# Legist — AI Assistant for NPA Comparison

Legist is a backend service for comparing revisions of legal acts (НПА) of the Republic of Belarus. Built for BSUIR Hackathon 2026.

## Tech Stack

- **Go 1.25** — backend
- **Echo v4** — HTTP framework
- **SQLite** (modernc, pure Go) + **sqlx** — database
- **Qdrant** — vector store for RAG (gRPC on 127.0.0.1:6334)
- **Ollama** (ROCm, gfx1035) — local LLM inference
  - `nomic-embed-text` — embeddings
  - `qwen2.5:3b` — metadata extraction
  - `qwen2.5:7b` — analysis and chat
- **NixOS** — deployment environment
- **Cloudflare Tunnel** — public access at `legist.nadevko.cc`

## Project Structure

```
legist/
├── api/                          # Go module (github.com/nadevko/legist)
│   ├── cmd/server/main.go        # entrypoint
│   └── internal/
│       ├── api/                  # Echo handlers, middleware, response types
│       │   ├── server.go         # Server struct, NewServer, registerRoutes
│       │   ├── auth.go           # /users, /sessions, /tokens handlers
│       │   ├── users.go          # password reset handlers
│       │   ├── files.go          # file upload, list, get, delete
│       │   ├── webhooks.go       # webhook endpoint CRUD + events
│       │   ├── chat.go           # /chat handler (stub)
│       │   ├── health.go         # /health
│       │   ├── doc.go            # swagger @annotations
│       │   ├── types.go          # response types, error handler, pagination params
│       │   ├── helpers.go        # bindListParams, listResult, ownerFilter, parseExpand
│       │   ├── expand.go         # expand[] middleware
│       │   ├── idempotency.go    # Idempotency-Key middleware
│       │   ├── writer.go         # bufferedWriter for middleware
│       │   └── id.go             # newID("prefix") generator
│       ├── auth/                 # JWT, bcrypt, middleware
│       │   ├── jwt.go            # NewAccessToken, ParseAccessToken
│       │   ├── middleware.go     # Middleware(secret), UserID(c), AuthError, Version
│       │   └── password.go       # HashPassword, CheckPassword
│       ├── config/               # env-based config
│       │   └── config.go         # Config struct, Load()
│       ├── pagination/           # cursor-based pagination
│       │   └── pagination.go     # Params, Page, Response, NewResponse
│       ├── parser/               # document parsing
│       │   ├── types.go          # Document, Section, Flatten, FlattenLeaves
│       │   ├── parser.go         # Parser interface, New(mime), ParseFile(path, mime)
│       │   ├── docx.go           # fumiama/go-docx parser
│       │   ├── pdf.go            # pdftotext via exec
│       │   ├── validate.go       # ValidateReader, Validate (magic bytes, size)
│       │   └── file.go           # readerAtFile helper
│       ├── sse/                  # Server-Sent Events
│       │   └── broker.go         # Broker, Subscribe, Publish, Stream, Format
│       ├── store/                # sqlx stores per resource
│       │   ├── db.go             # Open(path), schema, WAL pragmas
│       │   ├── models.go         # User, Session, File, PasswordReset, IdempotencyKey, WebhookEndpoint, WebhookEvent
│       │   ├── users.go          # UserStore: Create, GetByID, GetByEmail, UpdateEmail, UpdatePassword, Delete
│       │   ├── sessions.go       # SessionStore: Create, GetByTokenHash, ListByUser, DeleteByID, DeleteAllByUser
│       │   ├── files.go          # FileStore: Create, GetByID, List(filter, params), UpdateStatus, Delete
│       │   ├── password_resets.go# PasswordResetStore: Create, GetByTokenHash, Delete
│       │   ├── idempotency.go    # IdempotencyStore: Get, Lock, Set
│       │   └── webhooks.go       # WebhookStore: endpoint CRUD, events CRUD, ListEndpointsByEvent
│       └── webhook/              # webhook dispatcher
│           └── dispatcher.go     # Dispatcher, Dispatch, event type constants
├── nix/
│   ├── nixos/default.nix         # qdrant + ollama NixOS services
│   └── pkgs/                     # custom nix packages
├── shell.nix                     # dev shell (go, gopls, poppler_utils, etc.)
└── flake.nix                     # nix flake (gomod2nix, kasumi, apps.swagen)
```

## Conventions

### Language
- **Code and comments** — English only
- **Team communication** — Russian
- **Swagger annotations** — English

### Swagger
- Version locked at `v1-alpha` — never bump
- Single definition, no dropdown selector
- Generated via `nix run ..#swagen` (runs `swag init -g doc.go -d ./internal/api -o docs`, renames to `v1-alpha.json`)
- Served at `/swagger/v1-alpha.json` as static file
- Host hardcoded as `legist.nadevko.cc`

### File Storage
- Originals: `data/files/{file_id}/{filename}`
- Parsed JSON (planned): `data/files/{file_id}/parsed.json`
- Public laws (RAG base): `files.user_id IS NULL` in DB, populated by Python scripts
- User files: `files.user_id = <user_id>`

### SSE Events
All SSE events have shape: `{type: string, data: {...}}`
- `progress` — processing started
- `done` — completed successfully  
- `failed` — failed with error

### Idempotency
- Required for all POST requests
- Optional for PATCH and DELETE
- TTL: 24 hours (stored as `created_at`, expiry computed in SQL)
- Scoped by `(key, user_id)` — or IP for unauthenticated requests
- Returns 409 if same key is in-flight, 422 if reused for different endpoint

### Pagination
- Cursor: composite `(created_at DESC, id DESC)` — handles timestamp collisions
- `starting_after` — items after cursor (older)
- `ending_before` — items before cursor (newer)
- Default limit: 20, max: 100
- Store always fetches `limit+1`, caller slices and sets `has_more`

### Parser
- Validate magic bytes before saving to disk
- Max file size: 50MB
- DOCX: Word heading styles (`Heading 1/2/3`, `Заголовок 1/2/3`) define hierarchy
- PDF: regex patterns on pdftotext output define hierarchy
- Section IDs: hierarchical `s1`, `s1.2`, `s1.2.3`
- `Flatten()` — all sections DFS (for embeddings)
- `FlattenLeaves()` — leaf sections only (for Qdrant chunking)
Generated via `newID("prefix")` in `internal/api/id.go`:
```
file_a1b2c3d4e5f6   — files
user_a1b2c3d4e5f6   — users
sess_a1b2c3d4e5f6   — sessions
pwdr_a1b2c3d4e5f6   — password resets
whep_a1b2c3d4e5f6   — webhook endpoints
evt_<unix_nano>     — webhook events (dispatcher)
```
12 hex chars after prefix, sliced from UUID without dashes.

### API Style — Stripe
- Resources plural, ownership via JWT (no `/me/resource` paths, only `/me` alias)
- Every response: `id`, `object` (type string), `created` (unix timestamp)
- List: `{object: "list", data: [...], has_more: bool}`
- Delete: `{id, object, deleted: true}` with HTTP 200
- Errors: `{object: "error", error: {type, code, message, param?}}`
- Cursor pagination: `starting_after` / `ending_before` query params
- `expand[]=resource` — expand related objects inline
- `Idempotency-Key` header required for POST
- `Accept` header controls format AND mode (see below)

### Response Headers
- `Request-Id` — every response
- `Legist-Version: v1-alpha` — every response
- `Legist-Signature: sha256=...` — webhook deliveries only
- `Idempotency-Key` — echoed back if provided
- `Idempotent-Replayed: true` — if response was replayed from cache

### Content Negotiation via Accept
```
application/json       — JSON metadata (default)
text/event-stream      — SSE stream (progress or sync upload)
application/pdf        — file download
application/vnd...docx — file download
```

### Error Handling
- HTTP errors via `errorf(status, code, message, param?)` — never `echo.NewHTTPError` with raw string
- Auth errors via `*auth.AuthError` struct — handled in `errorHandler`
- Business logic errors via `fmt.Errorf("context: %w", err)`

### Database
- SQLite, WAL mode, `foreign_keys=ON`, `busy_timeout=5000`
- 3NF — no derived fields, no JSON columns (webhook events use separate table)
- Public laws: `files.user_id IS NULL`
- Cursor pagination: composite `(created_at, id)` cursor in all list queries

### Diff and RAG Architecture
- **Vectors (Qdrant)** — for RAG search only, not for diffing
- **Diff** — structural comparison of `[]Section` trees, matched by `Label`
- Word-level diff via `go-diff` for changed sections
- AI inference sequential (semaphore=1 for Ollama), RAG queries parallel
- Parsed `Document{}` saved as `parsed.json` to avoid re-parsing on each diff

### object Field Values
```
"user"                 — User
"session"              — Session  
"file"                 — File
"token"                — access/refresh token pair
"token.password_reset" — password reset token
"webhook.endpoint"     — WebhookEndpoint
"webhook.event"        — WebhookEvent delivery attempt
"list"                 — list response wrapper
"error"                — error response
"chat.completion"      — chat response (planned)
"diff"                 — diff result (planned)
"document"             — document group (planned)
"document.version"     — version within document (planned)
```

### NPA Hierarchy Levels (Qdrant payload)
```
0 — Constitution
1 — Republican referendum decisions
2 — Laws
3 — Presidential decrees and edicts
4 — Council of Ministers resolutions
5 — Parliament/Supreme Court/Prosecutor General NPAs
6 — Ministry NPAs
7 — Local NPAs
8 — Other regulatory bodies
9 — Technical NPAs
```

### Public (no auth)
```
GET    /                          → redirect to /swagger
GET    /health
GET    /swagger/*
POST   /users                     — register
POST   /sessions                  — login → {access_token, refresh_token}
POST   /tokens/refresh
POST   /tokens/password-reset     — returns reset_token directly (no email)
PATCH  /tokens/password-reset     — change password using reset_token
```

### Protected (Bearer token)
```
GET    /me                        — alias for GET /users/:currentUserID
PATCH  /me
DELETE /me

GET    /users/:id
PATCH  /users/:id                 — update email
DELETE /users/:id

GET    /sessions                  — list active sessions
DELETE /sessions/:id              — logout

GET    /files                     — own files (default) or ?owner=public for laws
GET    /files/:id                 — metadata / download / SSE (via Accept)
POST   /files                     — upload pdf/docx (sync or async via Accept)
DELETE /files/:id

POST   /webhooks
GET    /webhooks
GET    /webhooks/:id
GET    /webhooks/:id/events       — delivery history
PATCH  /webhooks/:id
DELETE /webhooks/:id

POST   /chat                      — RAG-based Q&A (stub)
```

### Planned (not yet implemented)
```
POST   /diffs
GET    /diffs
GET    /diffs/:id                 — status + SSE stream
GET    /diffs/:id/report          — JSON / DOCX / AKN XML via Accept

POST   /documents
GET    /documents
GET    /documents/:id
PATCH  /documents/:id
DELETE /documents/:id
POST   /documents/:id/versions
GET    /documents/:id/versions
GET    /documents/:id/versions/:num
DELETE /documents/:id/versions/:num
```

## File Processing Pipeline

```
POST /files (multipart)
  → magic bytes validation (before saving)
  → save to data/files/{file_id}/{filename}
  → store File{status: "pending"}
  → dispatch webhook: file.created
  → go parseFile()
      → UpdateStatus("processing")
      → SSE publish: progress
      → parser.ParseFile(path, mime)  →  Document{Sections[]}
          DOCX: fumiama/go-docx (heading styles → hierarchy)
          PDF:  pdftotext via exec (regex patterns → hierarchy)
      → on error: UpdateStatus("failed"), SSE failed, webhook file.failed
      → TODO: save parsed.json alongside original
      → UpdateStatus("done")
      → SSE publish: done
      → dispatch webhook: file.parsed
```

Tracking options:
1. `POST /files` + `Accept: text/event-stream` — sync, stream progress in response
2. `POST /files` + `Accept: application/json` — async, poll `GET /files/:id`
3. `GET /files/:id` + `Accept: text/event-stream` — subscribe at any time
4. Webhooks — `file.parsed` / `file.failed` events

## Diff Pipeline (planned)

```
[]Section (old doc)  ─┐
                      ├→ structural diff (match by Label)
[]Section (new doc)  ─┘
                           ↓
                      []Change{section, old_text, new_text}
                           ↓
              word-level diff (go-diff) for changed sections
                           ↓
              AI metadata (qwen2.5:3b): change_type, legal_impact
                           ↓
              RAG (embed → Qdrant search) for each change
                           ↓
              AI compliance (qwen2.5:7b): red_zones
                           ↓
              Report JSON / DOCX / AKN XML
```

Qdrant payload per chunk:
```json
{
  "text":    "section text",
  "source":  "document name",
  "article": "Статья 1",
  "level":   2
}
```
NPA hierarchy levels: 0=Constitution, 1=Referendum, 2=Laws, 3=Decrees, 4=Council of Ministers, 5=Parliament/Courts, 6=Ministries, 7=Local, 8=Other, 9=Technical

## Report Format (JSON)
```json
{
  "id": "uuid",
  "status": "done",
  "summary": "...",
  "diff": [
    {
      "section": "п. 3.1",
      "old_text": "...",
      "new_text": "...",
      "change_type": "semantic|wording|structural",
      "legal_impact": "...",
      "severity": "high|medium|low"
    }
  ],
  "red_zones": [
    {
      "section": "п. 3.1",
      "reference": "Закон №200-З, ст. 45, ч. 2",
      "explanation": "...",
      "severity": "high|medium|low",
      "level": 2
    }
  ]
}
```

## Webhooks

Events: `file.created`, `file.parsed`, `file.failed`, `file.deleted`, `diff.created`, `diff.done`, `diff.failed`, `user.created`, `user.deleted`

Delivery: 3 attempts with exponential backoff (1s, 4s). Signed with `Legist-Signature: sha256=HMAC(secret, body)`. Secret format: `whsec_...`.

## NixOS Services

```nix
services.ollama = {
  enable = true;
  package = pkgs.ollama-rocm;
  rocmOverrideGfx = "10.3.5";  # Ryzen 6000 iGPU (gfx1035)
  host = "127.0.0.1";
  port = 11434;
  loadModels = ["nomic-embed-text" "qwen2.5:3b" "qwen2.5:7b"];
};

services.qdrant = {
  enable = true;
  settings.service = { host = "127.0.0.1"; http_port = 6333; grpc_port = 6334; };
};
```

## Environment Variables

```bash
ADDR=:8080
ENV=dev                          # dev enables CORS
DB_PATH=../data/legist.sqlite
DATA_PATH=../data
JWT_SECRET=...
PUBLIC_HOST=legist.nadevko.cc    # swagger base URL

QDRANT_HOST=127.0.0.1
QDRANT_GRPC_PORT=6334

OLLAMA_BASE_URL=http://127.0.0.1:11434
EMBED_MODEL=nomic-embed-text
METADATA_MODEL=qwen2.5:3b
ANALYSIS_MODEL=qwen2.5:7b
LLM_METADATA=ollama
LLM_ANALYSIS=ollama              # prod: deepseek or anthropic

ANTHROPIC_API_KEY=
DEEPSEEK_API_KEY=
```

## Useful Commands

```bash
cd api

# run server
go run ./cmd/server/main.go

# generate swagger docs
nix run ..#swagen

# build
go build ./...

# enter dev shell
nix develop
```