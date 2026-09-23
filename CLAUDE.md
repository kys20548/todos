# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Go backend for a todo app (gin + viper + gorm + PostgreSQL), scaffolded from `template_golang_web` with `user` swapped for `todo`. Frontend is a TodoMVC (`javascript-es5`) build in `web/`, served by the same gin server. Layout follows the standard `cmd/`/`internal/`/`pkg/`/`third_party/` Go project convention; `pkg/` and `third_party/` are currently empty placeholders. No tests currently exist.

## Commands

```bash
docker compose up -d --build            # start Postgres + app + pgAdmin
docker compose run --rm app ./migrate up   # apply migrations — separate from app startup on purpose, see below
```

For local iteration without rebuilding the image:
```bash
docker compose up -d postgres   # start only Postgres (make postgres)
make migrateup                  # apply migrations (go run ./cmd/migrate up)
go run ./cmd/todoapp             # start server (make server) — must run from repo root, app.env/web/ are resolved relative to cwd
make test                       # go test -v -cover ./...
```

Server reads config from `app.env` via viper (`util.LoadConfig`); any field can be overridden by an environment variable of the same name (e.g. `DB_SOURCE`, `HTTP_SERVER_ADDRESS`).

## Architecture

```
cmd/todoapp/main.go     # load config → gorm.Open → db.NewStore → api.NewServer → graceful shutdown on SIGINT/SIGTERM
cmd/migrate/main.go     # standalone migration command (golang-migrate), not run as part of server startup
internal/api/            # gin handlers, router, middleware, unified response envelope
internal/errcode/        # business status codes
internal/db/              # gorm model (model.go) + hand-written Store interface/impl (store.go)
internal/db/migration/   # versioned .sql up/down files, go:embed'd into cmd/migrate
internal/util/           # viper config loading
web/                     # static frontend (TodoMVC), served directly by gin
pkg/, third_party/       # empty placeholders — nothing in this project needs them yet
```

`db.Store` is a small hand-written interface (`internal/db/store.go`) implemented by `GormStore`, which wraps a `*gorm.DB`. Handlers depend on `db.Store`, not the concrete struct, so it can be mocked in tests. The interface's method/param/result shapes were kept identical to the project's original sqlc-generated version on purpose, so switching the backing implementation didn't require touching `internal/api/todo.go` beyond its two `sql.ErrNoRows` → `gorm.ErrRecordNotFound` checks.

### Migration is a separate command, not part of server startup

`cmd/migrate` uses `golang-migrate`, not gorm's `AutoMigrate` — the data layer (`internal/db/store.go`, `model.go`) stays on gorm for queries, but schema changes are versioned `.sql` up/down files in `internal/db/migration/`. Those files are `go:embed`'d into the `migrate` binary itself (`internal/db/migration/embed.go`), not read off disk at runtime, so the binary is self-contained and the Docker image needs no extra `COPY` for them. The Docker image's `CMD` is just `./main` — no `entrypoint.sh` wrapper that runs migration before exec'ing the server. Reason: an app restart (crash, redeploy, `docker compose restart`) is not the same event as "schema changed," and coupling them means every ordinary restart re-runs migration and can fail for reasons that have nothing to do with the server itself. Run it explicitly: `docker compose run --rm app ./migrate up` in a container, `go run ./cmd/migrate up` locally, or just `./migrate up` if you have the binary — it's a plain CLI (`--help` works), not something that only makes sense wrapped in Make/compose.

Subcommands:
- `up` — apply all pending migrations.
- `down` — roll back every migration. **Destructive** (drops columns/tables) — never run against a real environment without a backup.
- `force <version>` — mark the current version without running any SQL. Needed once for any database whose schema was created a different way (e.g. an older deployment that used gorm `AutoMigrate`, which never wrote a `schema_migrations` row) — otherwise `up` tries to re-run `000001_init_schema` and fails on "relation already exists".

### Response envelope

Every response — success or error — is `{code, msg, data}` (`internal/api/response.go`). Handlers never `ctx.JSON` directly; they call `ok(ctx, data)` or `fail(ctx, httpStatus, errcode.Code, err)`. `err` is passed only for logging (via `ctx.Error()`), never serialized to the client — DB errors can leak table/column/parameter names, so callers must go through `request_id` + logs instead. Codes are defined in `internal/errcode/errcode.go` as `Code{ID, Msg}` values — code and message are declared together in one variable so they can't drift apart — segmented `E000` success, `E0xx` generic, `E1xx` todo-specific (`E2xx`/`E3xx` reserved for future modules). `Response` embeds `errcode.Code` anonymously so its `code`/`msg` fields flatten straight into the envelope. Only codes an actual branch returns are defined — don't add speculative codes.

### Middleware order

`requestIDMiddleware → httpLogger → gin.CustomRecovery(recoveryHandler)`, registered in that order in `internal/api/server.go`. This is load-bearing, not arbitrary:
- `requestIDMiddleware` must be outermost — every later layer's logger needs the id it sets.
- `recoveryHandler` must be innermost so a panic doesn't skip past `httpLogger`, which would silently drop the access log line for the one request that most needs it.

Log level follows HTTP status: 5xx → `error`, 4xx → `warn`, else `info`, so filtering to `error` means "server's own problems," not noise from bad user input. Use `getLogger(ctx)` inside handlers (not the global `log`) so lines carry `request_id`.

### Routing conventions

Collection-level bulk operations (`PATCH /todos` = complete/uncomplete all, `DELETE /todos?status=completed` = clear completed) are routed on the collection itself, not sub-paths like `/todos/complete-all` — a static segment and `:id` at the same level of gin's route tree panic on registration. `DELETE /todos` requires `status=completed` explicitly with no default and no `all` value, so a request missing the param can't wipe the table.

### Partial updates

`PATCH /todos/:id` fields are pointers (`*string`, `*bool`) to distinguish "field omitted" from "field sent as zero value" — otherwise `{"completed": false}` and `{}` are indistinguishable and "uncomplete" can never be expressed. Sending no fields at all is rejected with `ErrTodoNoFieldsToUpdate` rather than silently no-op-succeeding.

### Soft delete

`Todo.DeletedAt` is `gorm.DeletedAt`, not a boolean — one column answers both "deleted?" and "when?". gorm automatically scopes every query to `deleted_at IS NULL` and makes `Delete()` set the timestamp instead of removing the row, so a double-delete affects 0 rows (`SoftDeleteTodo` returns 0 `RowsAffected`, handler turns that into the same 404 as "never existed") rather than clobbering the original delete timestamp. API responses use `todoResponse` (not the raw `db.Todo` gorm struct) specifically to avoid leaking `gorm.DeletedAt`'s `{"Time":...,"Valid":false}` shape into the API contract.

### List and summary are separate endpoints

`GET /todos?status=&page_id=&page_size=` — all params optional with defaults (`status=all`, `page_id=1`, `page_size=50`, max 200). Response `data` is a plain array of items.

`GET /todos-summary` returns `{total, active, completed}` (always over *all* non-deleted todos, unaffected by any filter). Split into its own endpoint rather than embedded in the list response — the frontend footer only needs these three numbers and shouldn't force a filtered/paginated list query just to get them. Note the path is `/todos-summary`, not `/todos/summary`: the latter is a static segment that panics against `/todos/:id` in gin's route tree (same reason collection-level bulk ops live at `/todos`, not `/todos/complete-all`).
