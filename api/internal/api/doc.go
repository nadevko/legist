// Package api implements the HTTP server for Legist.
//
// @title           Legist API
// @version         v1-alpha
// @description     AI assistant for comparing revisions of normative legal acts (НПА) of the Republic of Belarus.
// @description
// @description     ## Authentication
// @description     All endpoints except public auth routes require a Bearer token in the `Authorization` header.
// @description     Obtain tokens via `POST /sessions`. Refresh via `POST /tokens/refresh`.
// @description
// @description     ## Request IDs
// @description     Every response includes a `Request-Id` header for debugging.
// @description
// @description     ## Idempotency
// @description     `POST` requests require an `Idempotency-Key` header.
// @description     Repeating a request with the same key returns the cached response.
// @description     Keys expire after 24 hours. Reusing a key for a different endpoint returns 422.
// @description
// @description     ## Roles
// @description     Users have `role`: `user` or `admin` (in `users` table and JWT `role` claim).
// @description     Only `admin` may change another user's role via `PATCH /users/:id`.
// @description     In dev (`ENV=dev`), new registrations default to `admin`; otherwise default is `user`.
// @description
// @description     ## Ownership and access control
// @description     Resources belong to the authenticated user. Attempting to read or mutate
// @description     another user's private resource returns 404 (not 403) to avoid leaking existence.
// @description     Public resources (`user_id IS NULL`) are readable by any authenticated user but cannot be mutated.
// @description     List `owner` query (Stripe-like): omit → non-admin sees only own rows; admin sees own + public.
// @description     `owner=null` or `owner=public` (aliases, admin only) → public rows only. `owner=<your user id>` → own rows only. Other ids are rejected.
// @description
// @description     ## Pagination
// @description     List endpoints support cursor-based pagination via `starting_after` and `ending_before`.
// @description     Default limit is 20, max 100. Response includes `has_more: true` if more items exist.
// @description
// @description     ## Expanding objects
// @description     Pass `expand[]=document` (or other resource name) to expand related objects inline.
// @description
// @description     ## Content negotiation
// @description     Use the `Accept` header to control response format:
// @description     - `application/json` — JSON metadata (default)
// @description     - `application/lessed` — parsed document structure and AKN metadata (when file status is `done`);
// @description       `chunk_content[]` stores chunk texts in DFS order; `sections[].chunks[]` keep positional metadata (`plain_start` / `plain_end`, section ids);
// @description       optional `chunk_embeddings[][]` (one vector per chunk in the same DFS order as `chunk_content`) and `embedding_model` when embedding completed;
// @description       each positional chunk metadata entry may contain `weight` used for weighted diff similarity
// @description     - `text/event-stream` — SSE stream (async progress or sync upload)
// @description     - `application/pdf`, `application/vnd...docx` — file download
// @description
// @description     ## Documents and files
// @description     A **Document** is the AKN Work-level entity — it identifies one НПА across all its versions.
// @description     A **File** is one physical version (редакция) of a Document.
// @description     Uploading via `POST /files` creates a new Document automatically.
// @description     Uploading via `POST /documents/:id/files` adds a version to an existing Document.
// @description     `GET /files?document_id=:id` is an alias for `GET /documents/:id/files`.
// @description     `POST /files` with `document_id` form field is an alias for `POST /documents/:id/files`.
// @description
// @description     ## File processing pipeline
// @description     Files are parsed asynchronously after upload. Track progress via:
// @description     1. **Sync SSE** — `POST /files` with `Accept: text/event-stream`
// @description     2. **Async SSE** — `GET /files/:id` with `Accept: text/event-stream`
// @description     3. **Parsed result** — `GET /files/:id` with `Accept: application/lessed` (available when status=done)
// @description     4. **Webhooks** — register an endpoint, receive `file.parsed` or `file.failed` events
// @description
// @description     ## Diffs
// @description     `POST /diffs` (multipart, `Idempotency-Key`) compares two file versions. Modes: **`left_file_id` + `right_file_id`**
// @description     (existing files, same document, both parsed); **one of those ids + `file`** (upload the other side); **`file_left` + `file_right`**
// @description     (upload both, new document; optional Work metadata fields). JSON response is minimal (`object: diff`, ids, `status`, `similarity_percent` when done).
// @description     `similarity_percent` is weighted similarity: `sum(sim_i*weight_left_i)/max(sumWeightsLeft,sumWeightsRight)*100`.
// @description     Use `expand[]=document`, `left_file`, `right_file` on list/get. `Accept: text/event-stream` on create or get streams until `diff_done` or `diff_failed`.
// @description
// @description     ### SSE progress stages
// @description     | Stage | Description |
// @description     |-------|-------------|
// @description     | `parsing_started` | Document structure parsing in progress |
// @description     | `llm_requested` | Metadata extraction starts as soon as first N chars are available |
// @description     | `llm_skipped` | All metadata was provided explicitly, LLM not needed |
// @description     | `llm_done` | LLM responded; `meta_score` and `meta_ok` fields present |
// @description     | `saving` | Writing plain and legist artifacts to disk |
// @description     | `embedding_started` | Chunk embedding via Ollama started (`chunks_total`) |
// @description     | `embedding` | Embedding progress (`embedding_percent`, `chunks_embedded`, `chunks_total`; throttled by `EMBED_PROGRESS_INTERVAL_MS`) |
// @description     | `embedding_done` | Embeddings written to legist JSON |
// @description     | `matching` | Diff chunk matching progress (cosine + greedy one-to-one); `embedding_percent` carries similarity progress based on `N_left` |
// @description     | `done` | Processing complete (parse + metadata + embeddings) |
// @description     | `failed` | Processing failed; `error` and `missing_fields` present |
// @description
// @description     ## AKN metadata
// @description     Metadata follows Akoma Ntoso conventions. Work-level fields (`subtype`, `number`,
// @description     `author`, `date`) are stored on the Document and are required for diff.
// @description     Expression-level fields (`version_date`, `version_label`, `language`, `pub_*`)
// @description     are stored on the File and are optional. If not supplied explicitly, the LLM
// @description     attempts to extract them from the document text. After upload, expression-level
// @description     fields can be corrected via `PATCH /files/:id`.
// @description
// @description     ## LLM prompts
// @description     Metadata extraction instructions are not hard-coded in Go: set `METADATA_LLM_PROMPT_FILE` to a text file path, or rely on the bundled default (`internal/config/metadata_prompt_default.txt` embedded at build time). Tune `METADATA_LLM_HTTP_TIMEOUT_MS` for Ollama latency.
// @description
// @description     ## Webhooks
// @description     Register endpoints via `POST /webhooks`. Each delivery is signed with HMAC-SHA256.
// @description     Verify the `Legist-Signature: sha256=...` header using your endpoint secret.
// @description     Failed deliveries are retried up to 3 times with exponential backoff (1s, 4s).
// @description     Inspect delivery history via `GET /webhooks/:id/events`.
// @description     Enable or disable an endpoint via `PATCH /webhooks/:id` with `{"enabled": false}`.
// @description
// @description     ### Supported webhook events
// @description     | Event | Description |
// @description     |-------|-------------|
// @description     | `file.created` | File uploaded |
// @description     | `file.parsed` | Parsing, metadata extraction, and chunk embedding (when required) succeeded |
// @description     | `file.failed` | Parsing, metadata extraction, or embedding failed |
// @description     | `file.deleted` | File deleted |
// @description     | `diff.created` | Diff job started |
// @description     | `diff.done` | Diff completed |
// @description     | `diff.failed` | Diff failed |
// @description     | `user.created` | User registered |
// @description     | `user.deleted` | User deleted |
// @description
// @description     ### Signature verification (Go example)
// @description     ```go
// @description     mac := hmac.New(sha256.New, []byte(secret))
// @description     mac.Write(body)
// @description     expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
// @description     ok := hmac.Equal([]byte(expected), []byte(signature))
// @description     ```
// @tag.name        Sessions
// @tag.description Registration, login, access-token refresh, logout, listing active sessions, and password-reset token flows.
//
// @tag.name        Users
// @tag.description User profiles: read and update your email, delete your account. Admins may read any user and assign `role` via PATCH.
//
// @tag.name        Documents
// @tag.description AKN Work-level documents: create, list, get, patch, delete; add and list file versions under `/documents/:id/files`.
//
// @tag.name        Files
// @tag.description File versions at `/files`: upload, list, get (JSON, application/lessed, binary, SSE), patch expression-level metadata, delete.
//
// @tag.name        Diffs
// @tag.description Compare two versions of the same document: multipart job creation, list, get, and SSE for async progress.
//
// @tag.name        Webhooks
// @tag.description Register webhook endpoints, choose events, inspect signed delivery history, enable or disable endpoints.
//
// @tag.name        Chat
// @tag.description RAG-based Q&A over legal corpora (stub).
//
// @tag.name        System
// @tag.description Liveness and routing helpers (e.g. health).
//
// @host            legist.nadevko.cc
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter token in format: Bearer {token}
package api
