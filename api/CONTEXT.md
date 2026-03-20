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
│       │   ├── files.go          # file upload, list, get, patch, delete; uses internal/upload for multipart
│       │   ├── diffs.go          # POST/GET /diffs; async diff pipeline + SSE; multipart (3 modes)
│       │   ├── documents.go      # document CRUD handlers
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
│       ├── parser/               # document parsing and metadata extraction
│       │   ├── types.go          # ParsedFile, ParsedMeta, Section, AKN types
│       │   ├── parser.go         # Parser interface, New(mime), ParseFile(path, mime)
│       │   ├── pipeline.go       # Run() — full parse+LLM+write pipeline with SSE progress
│       │   ├── meta.go           # ExtractMeta, LLM prompt, field validation, DeriveNPALevel
│       │   ├── docx.go           # fumiama/go-docx parser
│       │   ├── pdf.go            # pdftotext via exec
│       │   ├── validate.go       # ValidateReader, Validate (magic bytes, size)
│       │   └── file.go           # readerAtFile helper
│       ├── sse/                  # Server-Sent Events
│       │   └── broker.go         # Broker, Subscribe, Publish, Stream, Format
│       ├── upload/               # multipart parse, FormData (POST /files, POST /diffs)
│       │   └── upload.go
│       ├── store/                # sqlx stores per resource
│       │   ├── db.go             # Open(path), schema, WAL pragmas
│       │   ├── models.go         # User, Session, File, Document, Diff, PasswordReset,
│       │   │                     # IdempotencyKey, WebhookEndpoint, WebhookEvent
│       │   │                     # + ErrDuplicate, ErrNotOwner, IsUniqueViolation
│       │   ├── users.go          # UserStore: Create, GetByID, GetByEmail, UpdateEmail, UpdatePassword, Delete
│       │   ├── sessions.go       # SessionStore: Create, GetByTokenHash, ListByUser, DeleteByID, DeleteAllByUser
│       │   ├── files.go          # FileStore: Create, GetByID, List(filter, params), UpdateStatus, UpdateMeta, Delete
│       │   ├── documents.go      # DocumentStore: Create, GetByID, List, ApplyUpdate, Delete
│       │   ├── diffs.go          # DiffStore: Create, GetByID, List, UpdateStatus, UpdateResult
│       │   ├── password_resets.go# PasswordResetStore: Create, GetByTokenHash, Delete
│       │   ├── idempotency.go    # IdempotencyStore: Get, Lock, Set
│       │   └── webhooks.go       # WebhookStore: endpoint CRUD, events CRUD, ListEndpointsByEvent
│       └── webhook/              # webhook dispatcher
│           └── dispatcher.go     # Dispatcher, Dispatch, event type constants
├── nix/
│   ├── nixos/default.nix         # qdrant + ollama NixOS services
│   └── pkgs/                     # custom nix packages
├── shell.nix                     # dev shell (go, gopls, poppler_utils, etc.)
└── flake.nix                     # nix flake (gomod2nix, kasumi, apps.swagen, apps.tunnel)
```

## Conventions

### Language
- **Code and comments** — English only
- **Team communication** — Russian
- **Swagger annotations** — English

### Swagger
- Version locked at `v1-alpha` — never bump
- Single definition, no dropdown selector
- Generated via flake app `swagen` at **repository root** (parent of `api/`): `nix run .#swagen` from the repo root, or `nix run ..#swagen` from `api/`. Runs `swag init -g doc.go -d ./internal/api -o docs` inside `api/`, then renames `docs/swagger.json` → `docs/v1-alpha.json`.
- Served at `/swagger/v1-alpha.json` as static file
- Host hardcoded as `legist.nadevko.cc`

### File Storage
- Originals: `DATA_PATH/pdf/{file_id}` or `DATA_PATH/docx/{file_id}` (by mime)
- Source links: `DATA_PATH/sources/{file_id}` → symlink to file in `pdf/` or `docx/`
- Canonical plain text: `DATA_PATH/plain/{file_id}`
- Legistoso artifact JSON: `DATA_PATH/legistoso/{file_id}`
- Public laws (RAG base): `files.user_id IS NULL` in DB, populated by Python scripts
- User files: `files.user_id = <user_id>`

### SSE Events
All SSE events have shape: `{type: string, data: {...}}`

Progress stages during file processing:
- `parsing_started` — structure parsing started/completed (`sections_found` field)
- `llm_requested` — metadata extraction starts once first `LLM_METADATA_WINDOW` chars are available (`chars` field)
- `llm_skipped` — LLM not needed, all metadata provided explicitly
- `llm_done` — LLM responded (`meta_score`, `meta_ok` fields)
- `saving` — writing plain and legistoso artifacts
- `done` — completed successfully
- `failed` — failed with error (`error`, `missing_fields` fields)

Diff pipeline (channel = diff id; subscribe via `POST /diffs` or `GET /diffs/:id` with `Accept: text/event-stream`):
- `document_will_be_created` / `document_created` — only when uploading `file_left` + `file_right`
- `file_done` / `file_failed` — forwarded file-parse stages for pending uploads (same shape as file SSE)
- `diff_started` — computation begin (after files are ready)
- `diff_done` — job finished (`status=done`)
- `diff_failed` — job failed (`error` message)

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
- PDF: regex patterns on pdftotext output define hierarchy; path passed directly to avoid stdin issues
- Section IDs: hierarchical `s1`, `s1.2`, `s1.2.3`
- `Flatten()` — all sections DFS (for embeddings)
- `FlattenLeaves()` — leaf sections only (for Qdrant chunking)
- `MatchKey()` — stable diff key: `section_type:num` (survives renumbering)

### AKN Metadata
Documents follow Akoma Ntoso Work/Expression model:

**Work-level** (stored on `documents` table, required for diff):
- `subtype` — закон|кодекс|декрет|указ|постановление|приказ|решение|конституция|иное
- `number` — official document number, e.g. "296-З"
- `author` — issuing body, e.g. "Парламент"
- `date` — adoption date YYYY-MM-DD
- `npa_level` — derived deterministically from subtype+author (0–9)

**Expression-level** (stored on `files` table, optional):
- `version_date`, `version_number`, `version_label` — редакция
- `language` — rus|bel
- `pub_name`, `pub_date`, `pub_number` — publication info

**Enrichment** (stored in `parsed.json` only, not DB columns):
- `lifecycle` — generation/amendment/repeal events
- `passive_modifications` — what other acts changed this document
- `references` — TLC ontological entities
- `keywords` — subject tags

If metadata fields are not supplied at upload time, `qwen2.5:3b` attempts extraction from the first+last N chars of document text (`LLM_METADATA_WINDOW`). Required Work fields (subtype, number, date) missing after all retries → `file.status = failed`.

### API Style — Stripe
- Resources plural, ownership via JWT (no `/me/resource` paths, only `/me` alias)
- Every response: `id`, `object` (type string), `created` (unix timestamp)
- List: `{object: "list", data: [...], has_more: bool, next_cursor?: string}`
- Delete: `{id, object, deleted: true}` with HTTP 200
- Errors: `{object: "error", error: {type, code, message, param?}}`
- Cursor pagination: `starting_after` / `ending_before` query params
- `expand[]=resource` — expand related objects inline
- `Idempotency-Key` header required for POST

### Ownership and Security
- Resources return 404 (not 403) when they exist but belong to another user
- Public documents (`user_id IS NULL`) are readable by all but not mutable
- `DELETE /sessions/:id` uses `WHERE id=? AND user_id=?` — 0 rows = 404
- `POST /tokens/password-reset` always returns 200 regardless of email existence (anti-enumeration)
- `POST /users` returns 409 only on UNIQUE violation, other errors return 500

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
- Public laws: `files.user_id IS NULL`, `documents.user_id IS NULL`
- Cursor pagination: composite `(created_at, id)` cursor in all list queries
- `ErrDuplicate` — returned on UNIQUE constraint violation (detected via error string)
- `ErrNotOwner` — returned when DELETE/UPDATE affects 0 rows due to ownership mismatch

### Diff and RAG Architecture (planned)
- **Vectors (Qdrant)** — for RAG search only, not for diffing
- **Diff** — structural comparison of `[]Section` trees, matched by `MatchKey()` (type:num)
- Word-level diff via `go-diff` for changed sections
- AI inference sequential (semaphore=1 for Ollama), RAG queries parallel
- Parsed `Document{}` saved as `parsed.json` to avoid re-parsing on each diff

### object Field Values
```
"user"                 — User
"session"              — Session
"file"                 — File
"document"             — Document (AKN Work-level entity)
"token"                — access/refresh token pair
"token.password_reset" — password reset token
"webhook.endpoint"     — WebhookEndpoint
"webhook.event"        — WebhookEvent delivery attempt
"list"                 — list response wrapper
"error"                — error response
"chat.completion"      — chat response (planned)
"diff"                 — Diff job (`similarity_percent`, `status`; full diff payload planned for report)
```

### NPA Hierarchy Levels (npa_level)
Derived deterministically from `subtype` + `author` via `DeriveNPALevel()`:
```
0 — Constitution
1 — Republican referendum decisions
2 — Laws (закон, кодекс)
3 — Presidential decrees and edicts (декрет, указ)
4 — Council of Ministers resolutions (постановление, автор: СовМин)
5 — Parliament/Supreme Court/Prosecutor General NPAs
6 — Ministry NPAs (постановление/приказ, автор: министерство/комитет)
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
POST   /tokens/password-reset     — always 200 (anti-enumeration), token usable only if email exists
PATCH  /tokens/password-reset     — change password using reset_token
```

### Protected (Bearer token)
```
GET    /me                        — alias for GET /users/:currentUserID
PATCH  /me
DELETE /me

GET    /users/:id                 — own profile only (404 for others)
PATCH  /users/:id                 — own profile only
DELETE /users/:id                 — own profile only

GET    /sessions                  — list active sessions
DELETE /sessions/:id              — logout (404 if not found or not owner)

GET    /documents                 — list own documents; ?owner=public for public laws
POST   /documents                 — create document manually (409 on duplicate identity)
GET    /documents/:id
PATCH  /documents/:id             — update Work-level fields
DELETE /documents/:id

GET    /documents/:id/files       — list versions (canonical); alias: GET /files?document_id=:id; supports ?type=pdf|docx
POST   /documents/:id/files       — add version (canonical); alias: POST /files with document_id form field

GET    /files                     — own files; ?document_id= forwards to /documents/:id/files; supports ?type=pdf|docx
POST   /files                     — upload + create new Document (409 on duplicate identity)
GET    /files/:id                 — metadata / parsed artifact (`Accept: application/legistoso`) / download / SSE (via Accept)
PATCH  /files/:id                 — update Expression-level fields (version_date, language, pub_*, etc.)
DELETE /files/:id                 — 409 if status=processing

POST   /webhooks                  — URL and events validated; events must be from known list
GET    /webhooks
GET    /webhooks/:id
GET    /webhooks/:id/events       — delivery history
PATCH  /webhooks/:id              — update url, events, enabled toggle
DELETE /webhooks/:id

POST   /chat                      — RAG-based Q&A (stub)

POST   /diffs                     — multipart (Stripe-style): (1) `left_file_id` + `right_file_id` (same document, both `status=done`); (2) one id + `file` (upload second side); (3) `file_left` + `file_right` (new document; optional Work fields `subtype`, `number`, …). `Accept: text/event-stream` — SSE until `diff_done` / `diff_failed` (file stages reuse `file_done` / `file_failed` on diff channel)
GET    /diffs                     — list; `?document_id=`, `?file_id=` (left or right), `?status=`; `expand[]=document|left_file|right_file`
GET    /diffs/:id                 — JSON metadata; `Accept: text/event-stream` for SSE on same channel
```

### Planned (not yet implemented)
```
GET    /diffs/:id/report          — JSON / DOCX / AKN XML via Accept (export of diff result)
```

**Note:** `POST /diffs` currently runs a **placeholder** computation (`similarity_percent=0`, empty `diff_data`); structural/word diff and report export are future work.

## File Processing Pipeline

```
POST /files (multipart)
  → magic bytes validation (before saving)
  → save to data/files/{file_id}/{filename}
  → create Document{} if new (Work-level fields from form or empty)
  → store File{status: "pending", document_id}
  → dispatch webhook: file.created
  → go processFile(file, document)
      → UpdateStatus("processing")
      → SSE: parsing_started
      → parser.ParseFile(path, mime) → RawDocument{Sections[]}
          DOCX: fumiama/go-docx (heading styles → hierarchy)
          PDF:  pdftotext path (no stdin — more reliable across poppler versions)
      → SSE: parsing_started (sections_found)
      → needLLM = any required Work field empty OR any Expression field empty
      → if needLLM:
          → once first `LLM_METADATA_WINDOW` chars of plain text are ready: SSE llm_requested
          → go ExtractMeta(firstN + "---" + firstN)   # early async request
              → qwen2.5:3b via Ollama /api/generate
              → retry up to MetadataMaxRetries, merge best results
              → validate each field (length, date format, npaLevel range)
          → continue parsing document to the end regardless of LLM status
          → at finalization, wait for LLM response if still running
          → SSE: llm_done (meta_score, meta_ok)
      → else: SSE: llm_skipped
      → validate required Work fields (subtype, number, date)
      → on missing: UpdateStatus("failed"), SSE failed, webhook file.failed, return
      → DeriveNPALevel(subtype, author) — always deterministic, never from LLM
      → merge LLM results into Document (known fields win) → UpdateDocument
      → merge LLM results into File expression fields → UpdateFileMeta
      → assemble legistoso: {file_id, document_id, plain_text_path, plain_text_len, meta, sections[].chunks, parsed_at, parser_version}
      → SSE: saving
      → write DATA_PATH/plain/{file_id}
      → write DATA_PATH/legistoso/{file_id}
      → UpdateStatus("done")
      → SSE: done
      → dispatch webhook: file.parsed
```

Tracking options:
1. `POST /files` + `Accept: text/event-stream` — sync, stream progress in response
2. `POST /files` + `Accept: application/json` — async, poll `GET /files/:id`
3. `GET /files/:id` + `Accept: text/event-stream` — subscribe at any time
4. `GET /files/:id` with `Accept: application/legistoso` — fetch legistoso artifact when done
5. Webhooks — `file.parsed` / `file.failed` events

## Diff Pipeline (planned)

```
[]Section (old doc)  ─┐
                      ├→ structural diff (match by MatchKey = type:num)
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

Delivery: 3 attempts with exponential backoff (1s, 4s). Signed with `Legist-Signature: sha256=HMAC(secret, body)`. Secret format: `whsec_...`. Enable/disable via `PATCH /webhooks/:id` with `{"enabled": false/true}`.

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

LLM_METADATA_WINDOW=3000         # chars of document text sent to metadata LLM
LLM_METADATA_RETRIES=3           # max LLM attempts per file

ANTHROPIC_API_KEY=
DEEPSEEK_API_KEY=
```

## ID Format
Generated via `newID("prefix")` in `internal/api/id.go`:
```
doc_a1b2c3d4e5f6    — documents
file_a1b2c3d4e5f6   — files
diff_a1b2c3d4e5f6   — diffs
user_a1b2c3d4e5f6   — users
sess_a1b2c3d4e5f6   — sessions
pwdr_a1b2c3d4e5f6   — password resets
whep_a1b2c3d4e5f6   — webhook endpoints
evt_<unix_nano>     — webhook events (dispatcher)
```
12 hex chars after prefix, sliced from UUID without dashes.

## Useful Commands

```bash
cd api

# run server
go run ./cmd/server/main.go

# generate swagger docs (flake at repo root)
cd .. && nix run .#swagen
# or from api/: nix run ..#swagen

# build
go build ./...

# enter dev shell
nix develop
```