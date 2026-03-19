// Package api implements the HTTP server for Legist.
//
// @title           Legist API
// @version         v1-alpha
// @description     AI assistant for comparing revisions of legal acts of the Republic of Belarus.
// @description
// @description     ## Authentication
// @description     All endpoints except auth require a Bearer token in the Authorization header.
// @description     Obtain tokens via `POST /sessions`. Refresh via `POST /tokens/refresh`.
// @description
// @description     ## Request IDs
// @description     Every response includes a `Request-Id` header for debugging.
// @description
// @description     ## Idempotency
// @description     `POST` requests require an `Idempotency-Key` header.
// @description     Repeating a request with the same key returns the cached response.
// @description     Keys expire after 24 hours. If a key is reused for a different
// @description     endpoint, a 422 error is returned.
// @description
// @description     ## Pagination
// @description     List endpoints support cursor-based pagination via `starting_after`
// @description     and `ending_before` query params. Default limit is 20, max 100.
// @description     Response includes `has_more: true` if more items exist.
// @description
// @description     ## Expanding objects
// @description     Pass `expand[]=user` (or other resource name) to expand related
// @description     objects inline instead of returning only their ID.
// @description
// @description     ## Content negotiation
// @description     Use the `Accept` header to control response format and behaviour:
// @description     - `application/json` — JSON metadata (default)
// @description     - `text/event-stream` — SSE stream (async progress or sync upload)
// @description     - `application/pdf`, `application/vnd...docx` — file download
// @description
// @description     ## File processing
// @description     Files are parsed asynchronously after upload. Track progress via:
// @description     1. **Webhooks** — register an endpoint, receive `file.parsed` event
// @description     2. **Sync SSE** — `POST /files` with `Accept: text/event-stream`
// @description     3. **Async SSE** — `GET /files/:id` with `Accept: text/event-stream`
// @description
// @description     ## Webhooks
// @description     Register endpoints via `POST /webhooks`. Each delivery is signed
// @description     with HMAC-SHA256. Verify the `Legist-Signature: sha256=...` header
// @description     using your endpoint secret. Failed deliveries are retried 3 times
// @description     with exponential backoff. Inspect history via `GET /webhooks/:id/events`.
// @description
// @description     ### Supported events
// @description     | Event | Description |
// @description     |-------|-------------|
// @description     | `file.created` | File uploaded |
// @description     | `file.parsed` | Parsing succeeded |
// @description     | `file.failed` | Parsing failed |
// @description     | `file.deleted` | File deleted |
// @description     | `diff.created` | Diff started |
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
// @host            legist.nadevko.cc
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter token in format: Bearer {token}
package api
