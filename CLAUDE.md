# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Go backend for a todo app (gin + viper + gorm + PostgreSQL), scaffolded from `template_golang_web` with `user` swapped for `todo`. Frontend is a TodoMVC (`javascript-es5`) build in `web/`, served by the same gin server. Layout follows the standard `cmd/`/`internal/`/`pkg/`/`third_party/` Go project convention; `pkg/` and `third_party/` are currently empty placeholders. No tests currently exist.

## Commands

The app is one binary, `todoapp`, with three subcommands: `serve`, `migrate`, `worker`. All three run from the same Docker image and are told apart only by compose `command:`.

```bash
docker compose -f docker-compose.local.yaml up -d --build   # laptop: db + migrate + serve + worker + pgAdmin
```

For local iteration without rebuilding the image:
```bash
make postgres                   # start only the db from docker-compose.local.yaml
make migrateup                  # apply migrations (go run ./cmd/todoapp --env dev migrate up)
make server                     # go run ./cmd/todoapp --env dev serve — must run from repo root, config/ and web/ are resolved relative to cwd
go run ./cmd/todoapp worker event   # demo worker: logs todo counts every --interval (default 30s), exits cleanly on SIGTERM
make test                       # go test -v -cover ./...
```

### Compose files

- `docker-compose.yaml` — deployed on the **jump** host: `migrate` (`restart: "no"`), `serve` and `worker` (`unless-stopped`), sharing one image. `DB_SOURCE` is required (from `.env`) and points at the db host's internal IP; across machines a compose service name does not resolve.
- `deploy/db/docker-compose.yaml` — deployed on the **devbox** (internal, no internet): only postgres, port bound to the internal IP, not `0.0.0.0`.
- `docker-compose.local.yaml` — laptop only, adds a `db` service and pgAdmin.
- `.env.example` documents the variables; `.env` is gitignored.
- Offline delivery: `docker save` → copy via the jump host → `docker load`. Build output goes in `dist/` (gitignored).

Config is per-environment: `config/app.{dev,qa,prod}.env`, selected by the global `--env <name>` flag (or `APP_ENV`), placed before the subcommand (`todoapp --env qa serve`), defaulting to `dev`. Makefile passes `ENV` (`make server ENV=qa`); compose passes `APP_ENV` (`APP_ENV=qa docker compose up -d`). `util.LoadConfig(path, env)` rejects any name outside `dev`/`qa`/`prod` so a typo fails fast instead of silently loading nothing. `Config.Environment` is set from that `env` argument, not read from the file — there's no `ENVIRONMENT` key — so the flag and the file can't disagree; `dev` means text logs + gin debug mode, anything else JSON logs + release mode. Any field can still be overridden by an environment variable of the same name (e.g. `DB_SOURCE`, `HTTP_SERVER_ADDRESS`).

## Architecture

```
cmd/todoapp/main.go     # root urfave/cli command, global --env flag, registers the three subcommands
cmd/todoapp/serve.go    # serve [--port]: load config → gorm.Open → db.NewStore → api.NewServer → graceful shutdown on SIGINT/SIGTERM
cmd/todoapp/migrate.go  # migrate up|down|force (golang-migrate), a one-shot command, not run as part of server startup
cmd/todoapp/worker.go   # worker event: demo background worker (periodic todo count log)
cmd/todoapp/common.go   # logger setup + gorm.Open shared by serve and worker
internal/api/            # gin handlers, router, middleware, unified response envelope
internal/errcode/        # business status codes
internal/db/              # gorm model (model.go) + hand-written Store interface/impl (store.go)
internal/db/migration/   # versioned .sql up/down files, go:embed'd into the todoapp binary
internal/util/           # viper config loading
web/                     # static frontend (TodoMVC), served directly by gin
pkg/, third_party/       # empty placeholders — nothing in this project needs them yet
```

`db.Store` is a small hand-written interface (`internal/db/store.go`) implemented by `GormStore`, which wraps a `*gorm.DB`. Handlers depend on `db.Store`, not the concrete struct, so it can be mocked in tests. The interface's method/param/result shapes were kept identical to the project's original sqlc-generated version on purpose, so switching the backing implementation didn't require touching `internal/api/todo.go` beyond its two `sql.ErrNoRows` → `gorm.ErrRecordNotFound` checks.

### Migration is a separate command, not part of server startup

`todoapp migrate` uses `golang-migrate`, not gorm's `AutoMigrate` — the data layer (`internal/db/store.go`, `model.go`) stays on gorm for queries, but schema changes are versioned `.sql` up/down files in `internal/db/migration/`. Those files are `go:embed`'d into the `todoapp` binary itself (`internal/db/migration/embed.go`), not read off disk at runtime, so the binary is self-contained and the Docker image needs no extra `COPY` for them. The Docker image's `ENTRYPOINT` is `./todoapp` with default `CMD ["serve","--port","8080"]` — no `entrypoint.sh` wrapper that runs migration before exec'ing the server; in compose, `migrate` is its own service with `restart: "no"` and `serve`/`worker` wait for it via `condition: service_completed_successfully`. Reason: an app restart (crash, redeploy, `docker compose restart`) is not the same event as "schema changed," and coupling them means every ordinary restart re-runs migration and can fail for reasons that have nothing to do with the server itself. Run it explicitly: `docker compose run --rm migrate` in a container, `make migrateup` / `go run ./cmd/todoapp migrate up` locally, or just `./todoapp migrate up` if you have the binary — it's a plain CLI (`--help` works), not something that only makes sense wrapped in Make/compose.

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
