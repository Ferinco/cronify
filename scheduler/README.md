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
`CRONIFY_WEBHOOK_URL` (if set, a job whose run exhausts every attempt POSTs
a `{"event":"job.failed", "jobId", "source", "route", "appUrl", "runId",
"attempts", "error"}` JSON body here; delivery is best-effort — a broken
webhook endpoint is logged, never affects the run's own bookkeeping or
retry behavior).

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

Or skip the local build entirely and pull the prebuilt multi-arch
(amd64/arm64) image: CI
([`.github/workflows/publish-scheduler-image.yml`](../.github/workflows/publish-scheduler-image.yml))
publishes it to GHCR on every push to `main` that touches `scheduler/`, plus
on version tags:

```sh
docker run -p 8080:8080 -e CRONIFY_ADMIN_TOKEN=<token> -v cronify-data:/data ghcr.io/<owner>/cronify-scheduler:latest
```

### Render — real one-click button

The repo-root [`render.yaml`](../render.yaml) Blueprint + the badge in the
[repo README](../README.md) work with zero setup beyond the repo being
public — Render reads `render.yaml`, builds this Dockerfile, and prompts for
`CRONIFY_ADMIN_TOKEN` at deploy time. `main`'s `render.yaml` uses `plan:
starter` with a persistent disk mounted at `/data` — the real default,
since without it `cronify.db` (and all job/run history) is wiped on every
restart or redeploy.

**No-cost trial variant:** the `render-trial-free` branch has a `plan: free`
`render.yaml` with the `disk` block removed — Render's free instances don't
support persistent disks, so this deploys with no billing info attached, at
the cost of job/run history not surviving restarts. Use it to kick the tires
before committing to a paid plan: deploy that branch's `render.yaml` via
Render's Blueprint flow (or point the "Deploy to Render" flow at that branch
instead of `main`), then switch to `main`'s `starter` + disk config once
you're ready for durable storage.

Render is the only deploy target this repo carries config for — Railway and
Fly.io configs (`railway.json`, `fly.toml`) were dropped since Render's
badge was the only one of the three that actually worked with zero setup
(no account-linked step, see git history pre-dating this if reviving
either). Nothing stops deploying this image anywhere else Docker runs — the
`docker build`/`docker run` commands above work unchanged on Railway, Fly,
or any other host, there just isn't a maintained platform-specific config
or badge for them here.
