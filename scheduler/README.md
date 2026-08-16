# scheduler

The self-hosted Go scheduler: tick loop, SQLite storage, retry/backoff,
overlap-protection locking, the `/api/v1/*` HTTP API that `cronify sync`
talks to, and a bundled server-rendered HTML dashboard — all one binary, one
process, one port. See [`../SPEC.md`](../SPEC.md) and
[`../CLAUDE.md`](../CLAUDE.md) for the full design this is built against —
the approved API contract, retry policy numbers, locking schema, and
implementation notes (SQLite driver choice, cron parsing, tick-loop
concurrency model, dashboard auth/routing, etc.) all live there.

Docker packaging + one-click deploy buttons is the only remaining
build-order step, per `SPEC.md`.

## Run it

```sh
CRONIFY_ADMIN_TOKEN=<token> go run .
```

Then visit `http://localhost:8080/` for the dashboard (any username, the
token as the password — your browser will prompt) or call `/api/v1/*` with
`Authorization: Bearer <token>`.

Other env vars (all optional, see CLAUDE.md's "Env var naming" for the full
list and defaults): `CRONIFY_PORT`, `CRONIFY_DB_PATH`,
`CRONIFY_TICK_INTERVAL_SECONDS`, `CRONIFY_DEFAULT_TIMEOUT_SECONDS`,
`CRONIFY_DEFAULT_MAX_ATTEMPTS`, `CRONIFY_STALE_LOCK_TIMEOUT_SECONDS`,
`CRONIFY_WEBHOOK_URL` (loaded but not yet acted on — failure alerting isn't
built yet).

## Build / test

```sh
go build ./...
go vet ./...
go test ./...
```
