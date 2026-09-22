# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Go backend for a todo app (gin + viper + sqlc + PostgreSQL), scaffolded from `template_golang_web` with `user` swapped for `todo`. No frontend yet (planned: todomvc in `web/`). Backend only, no tests currently exist.

## Commands

```bash
docker compose up -d          # start PostgreSQL (make postgres)
make migrateup                # apply migrations (requires golang-migrate CLI)
make migratedown               # roll back migrations
go run main.go                 # start server (make server)
make sqlc                      # regenerate db/sqlc/ from db/query/*.sql after editing queries
make test                      # go test -v -cover ./...
```

On Windows without `make`/`migrate` CLI, pipe SQL into the container directly instead:
```powershell
Get-Content db\migration\000001_init_schema.up.sql | docker exec -i todoapp_db psql -U root -d todoapp
Get-Content db\migration\000002_add_soft_delete.up.sql | docker exec -i todoapp_db psql -U root -d todoapp
```

Server reads config from `app.env` via viper (`util.LoadConfig`); any field can be overridden by an environment variable of the same name (e.g. `DB_SOURCE`, `HTTP_SERVER_ADDRESS`).

## Architecture

```
main.go        # load config → open DB → db.NewStore → api.NewServer → graceful shutdown on SIGINT/SIGTERM
api/            # gin handlers, router, middleware, unified response envelope
errcode/        # business status codes
db/migration/   # golang-migrate SQL, also doubles as sqlc's schema source
db/query/       # sqlc query definitions (.sql)
db/sqlc/        # sqlc-generated code + hand-written Store interface (store.go)
util/           # viper config loading
```

`db.Store` embeds the sqlc-generated `Querier` interface (`db/sqlc/store.go`); `SQLStore` wraps a `*sql.DB` and adds `execTx` for transactions. Handlers depend on `db.Store`, not the concrete struct, so it can be mocked in tests.

### Response envelope

Every response — success or error — is `{code, msg, data}` (`api/response.go`). Handlers never `ctx.JSON` directly; they call `ok(ctx, data)` or `fail(ctx, httpStatus, errcode.Code, err)`. `err` is passed only for logging (via `ctx.Error()`), never serialized to the client — DB errors can leak table/column/parameter names, so callers must go through `request_id` + logs instead. Codes are defined in `errcode/errcode.go`, segmented `0` success, `1xxxx` generic, `4xxxx` todo-specific (`2xxxx`/`3xxxx` reserved for future modules). Only codes an actual branch returns are defined — don't add speculative codes.

### Middleware order

`requestIDMiddleware → httpLogger → gin.CustomRecovery(recoveryHandler)`, registered in that order in `api/server.go`. This is load-bearing, not arbitrary:
- `requestIDMiddleware` must be outermost — every later layer's logger needs the id it sets.
- `recoveryHandler` must be innermost so a panic doesn't skip past `httpLogger`, which would silently drop the access log line for the one request that most needs it.

Log level follows HTTP status: 5xx → `error`, 4xx → `warn`, else `info`, so filtering to `error` means "server's own problems," not noise from bad user input. Use `getLogger(ctx)` inside handlers (not the global `log`) so lines carry `request_id`.

### Routing conventions

Collection-level bulk operations (`PATCH /todos` = complete/uncomplete all, `DELETE /todos?status=completed` = clear completed) are routed on the collection itself, not sub-paths like `/todos/complete-all` — a static segment and `:id` at the same level of gin's route tree panic on registration. `DELETE /todos` requires `status=completed` explicitly with no default and no `all` value, so a request missing the param can't wipe the table.

### Partial updates

`PATCH /todos/:id` fields are pointers (`*string`, `*bool`) to distinguish "field omitted" from "field sent as zero value" — otherwise `{"completed": false}` and `{}` are indistinguishable and "uncomplete" can never be expressed. Sending no fields at all is rejected with `ErrTodoNoFieldsToUpdate` rather than silently no-op-succeeding.

### Soft delete

`todos.deleted_at timestamptz` (migration 000002), not a boolean — one column answers both "deleted?" and "when?". Every query (get/list/update) filters `deleted_at IS NULL`; the delete queries themselves also filter on it, so a double-delete affects 0 rows and returns the same 404 as "never existed" rather than clobbering the original delete timestamp. API responses use `todoResponse` (not the raw `db.Todo` sqlc struct) specifically to avoid leaking `sql.NullTime`'s `{"Time":...,"Valid":false}` shape into the API contract.

### List and summary are separate endpoints

`GET /todos?status=&page_id=&page_size=` — all params optional with defaults (`status=all`, `page_id=1`, `page_size=50`, max 200). Response `data` is a plain array of items.

`GET /todos-summary` returns `{total, active, completed}` (always over *all* non-deleted todos, unaffected by any filter). Split into its own endpoint rather than embedded in the list response — the frontend footer only needs these three numbers and shouldn't force a filtered/paginated list query just to get them. Note the path is `/todos-summary`, not `/todos/summary`: the latter is a static segment that panics against `/todos/:id` in gin's route tree (same reason collection-level bulk ops live at `/todos`, not `/todos/complete-all`).
