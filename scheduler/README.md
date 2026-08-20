# scheduler

The self-hosted Go scheduler: tick loop, SQLite storage, retry/backoff,
overlap-protection locking, the `/api/v1/*` HTTP API that `cronify sync`
talks to, and a bundled server-rendered HTML dashboard — all one binary, one
process, one port. See [`../SPEC.md`](../SPEC.md) and
[`../CLAUDE.md`](../CLAUDE.md) for the full design this is built against —
the approved API contract, retry policy numbers, locking schema, and
implementation notes (SQLite driver choice, cron parsing, tick-loop
concurrency model, dashboard auth/routing, etc.) all live there.

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

## Deploy

A `Dockerfile` here builds a single static, non-root, ~15-20MB image
(`gcr.io/distroless/static-debian12:nonroot` — no shell, so healthchecks are
configured at the platform level against `GET /healthz`, not via a
Dockerfile `HEALTHCHECK`). `CRONIFY_DB_PATH` defaults to `/data/cronify.db`
in the image — mount a volume at `/data` for persistence.

```sh
docker build -t cronify-scheduler .
docker run -p 8080:8080 -e CRONIFY_ADMIN_TOKEN=<token> -v cronify-data:/data cronify-scheduler
```

Or from the repo root: `docker compose up -d` (`docker-compose.yml` reuses
this Dockerfile, reads `CRONIFY_ADMIN_TOKEN` from the environment).

### Render — real one-click button

The repo-root [`render.yaml`](../render.yaml) Blueprint + the badge in the
[repo README](../README.md) work with zero setup beyond the repo being
public — Render reads `render.yaml`, builds this Dockerfile, and prompts for
`CRONIFY_ADMIN_TOKEN` at deploy time.

### Railway — config ready, real badge needs one manual step

[`railway.json`](railway.json) gives correct build/deploy settings the
moment you connect the repo: **New Project → Deploy from GitHub repo → set
Root Directory to `scheduler`**, then add `CRONIFY_ADMIN_TOKEN` and a volume
mounted at `/data` in the Railway dashboard. A real "Deploy on Railway"
badge additionally requires *publishing a Railway Template* from your own
account (Railway dashboard → Templates → generate from this project →
Publish) — that's an account-linked action only you can do, so it's not
wired up automatically here.

### Fly.io — config ready, no stable badge mechanism exists

Fly doesn't currently have a Render-style static README badge. Deploy via
their CLI instead:

```sh
fly launch --no-deploy   # confirms the app without overwriting fly.toml
fly secrets set CRONIFY_ADMIN_TOKEN=<token>
fly deploy
```

[`fly.toml`](fly.toml) pins `auto_stop_machines = "off"` and
`min_machines_running = 1` deliberately — Fly's default scale-to-zero would
kill the tick loop, defeating the entire point of this being an always-on
process.
