# scheduler

The self-hosted Go scheduler: tick loop, SQLite storage, retry/backoff,
overlap-protection locking, and the `/api/v1/*` HTTP API that `cronify sync`
and (eventually) the dashboard talk to. See [`../SPEC.md`](../SPEC.md) and
[`../CLAUDE.md`](../CLAUDE.md) for the full design this is built against —
the approved API contract, retry policy numbers, locking schema, and
implementation notes (SQLite driver choice, cron parsing, tick-loop
concurrency model, etc.) all live there.

The bundled dashboard (server-rendered HTML, same binary) is not built yet —
next in the build order per `SPEC.md`.

## Run it

```sh
CRONIFY_ADMIN_TOKEN=<token> go run .
```

Other env vars (all optional, see CLAUDE.md's "Env var naming" for the full
list and defaults): `CRONIFY_PORT`, `CRONIFY_DB_PATH`,
`CRONIFY_TICK_INTERVAL_SECONDS`, `CRONIFY_DEFAULT_TIMEOUT_SECONDS`,
`CRONIFY_DEFAULT_MAX_ATTEMPTS`, `CRONIFY_STALE_LOCK_TIMEOUT_SECONDS`,
`CRONIFY_WEBHOOK_URL`.

## Build / test

```sh
go build ./...
go vet ./...
go test ./...
```
