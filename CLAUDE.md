# CLAUDE.md

Guidance for whoever (human or Claude) picks this repo up next. Full product
rationale lives in [`SPEC.md`](SPEC.md) — this file is the condensed,
implementation-facing version plus the decisions that resolved SPEC.md's
"Open technical questions" and the current build status.

## What this is

Cronify is a self-hosted scheduling layer for Next.js apps on serverless
platforms (Vercel, Netlify, etc.). Free pingers (cron-job.org, GitHub
Actions) can hit a URL on a schedule but don't retry failures or prevent a
slow job's previous run from overlapping with the next one. Cronify adds
retries + locking + a dashboard, self-hosted so nobody depends on a third
party's uptime.

Four pieces, polyglot repo (not an npm workspace — each piece builds
independently):

```
cronify/
├── packages/cronify/   TypeScript — defineJob(), route generation, CLI  [BUILT]
├── scheduler/          Go — tick loop, SQLite, retries, locking, API, dashboard [NOT STARTED]
├── site/                Next.js — marketing one-pager [NOT STARTED]
├── SPEC.md
└── CLAUDE.md
```

Build order (each step independently useful, per SPEC.md): (1) `defineJob()`
+ route generation, (2) standalone `withLock()` primitive, (3) the scheduler,
(4) the dashboard, (5) Docker + one-click deploy buttons. **Currently at the
end of step 2** — `packages/cronify` (including `withLock()`) is built and
tested; scheduler, dashboard, and packaging have not been started.

## Resolved design decisions

These were open questions in SPEC.md, proposed and approved before any code
was written. Treat them as settled unless the user says otherwise.

### Manifest format (`.cron-manifest.json`, project root)

```json
{
  "version": 1,
  "generatedAt": "2026-08-16T08:00:00.000Z",
  "jobs": [
    {
      "id": "daily-report",
      "schedule": "0 9 * * *",
      "route": "/api/cron/daily-report",
      "enabled": true,
      "timeoutSeconds": 30,
      "maxAttempts": 3,
      "description": null
    }
  ]
}
```

Deliberately has no host/`appUrl` — that's supplied at `sync` time
(`--app-url` / `CRONIFY_APP_URL`), so the manifest is identical across
environments.

### API contract (Go scheduler, not yet built)

All under `/api/v1/*`, auth via `Authorization: Bearer <CRONIFY_ADMIN_TOKEN>`:

- `POST /api/v1/sync` — body `{ source, appUrl, jobs: [...] }`. Full
  reconciliation for that `source`: upserts jobs present, deletes jobs
  missing. This is what `cronify sync` calls — one call, no client-side
  diffing. `source` lets one scheduler serve multiple apps; the CLI defaults
  it to the app's `package.json` `"name"`.
- `GET /api/v1/jobs`, `GET /api/v1/jobs/:id`, `GET /api/v1/jobs/:id/runs`
- `POST /api/v1/jobs/:id/run` — manual trigger
- `POST /api/v1/jobs/:id/pause` / `/resume`
- `DELETE /api/v1/jobs/:id`
- `GET /healthz`

### Retry policy, numerically

- Default `maxAttempts: 3` (1 initial + 2 retries), overridable per job.
- Failure = non-2xx response, timeout, or connection error. Default
  per-attempt timeout 30s (`timeoutSeconds`), overridable per job.
- Backoff: exponential, base 30s, ×2 per attempt, ±20% jitter, capped at 5
  min (retry 1 ≈30s later, retry 2 ≈60s later).
- Retries are driven by the scheduler's own timer, not the next minute-tick.

### Locking, concretely (SQLite, scheduler-side)

```sql
CREATE TABLE job_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  job_id TEXT NOT NULL,
  status TEXT NOT NULL, -- in_progress | success | failed
  attempt INTEGER NOT NULL DEFAULT 1,
  started_at DATETIME NOT NULL,
  finished_at DATETIME,
  http_status INTEGER,
  error TEXT
);
```

Before firing, check for a row with `status='in_progress'` for that job
inside a `BEGIN IMMEDIATE` transaction (avoids a race with the dashboard's
manual "run now"). If found and `now - started_at < staleLockTimeoutSeconds`
→ skip this tick. If found and older → mark it `failed` (`error: "stale
lock"`), reclaim, and proceed. Default `staleLockTimeoutSeconds: 600`, must
be ≥ the job's timeout, configurable per job.

### Secret/auth mechanism

Two separate secrets — do not conflate them:

- **Scheduler → Next.js route**: `Authorization: Bearer <CRON_SECRET>`,
  exact match via constant-time comparison (`node:crypto.timingSafeEqual`).
  No HMAC — no meaningful payload to sign for a trigger-only POST, and it
  matches Vercel Cron's own convention. Implemented in
  `packages/cronify/src/server.ts`.
- **CLI/dashboard → scheduler admin API**:
  `Authorization: Bearer <CRONIFY_ADMIN_TOKEN>`, env var on the scheduler.

### Next.js routing convention

App Router, **static per-job routes**, not a literal `[jobId]` dynamic
segment: `cronify generate` writes `app/api/cron/<jobId>/route.ts` (or
`src/app/...`) per job — a thin, regenerable wrapper importing the job and
`createRouteHandler` from `cronify/server`. All real logic (secret check,
handler invocation, response shape) lives in the package, not the generated
file. A dynamic `[jobId]` route would need a runtime job registry for no
real benefit here.

Job discovery convention (fixed for v1, no config file): files under
`cron/**/*.ts` at the project root, each `export default defineJob(...)`
**called directly** (not assigned to a variable first, not a named export).
`id` and `schedule` — and any other `defineJob()` option — must be static
literals: discovery uses the TypeScript compiler API to read them via AST
and **never executes the file**, so it never needs the handler's runtime
imports or env vars just to generate routes/manifest.

### Env var naming

- App side: `CRON_SECRET` (matches Vercel Cron's own convention)
- Scheduler side: `CRONIFY_ADMIN_TOKEN`, `CRONIFY_PORT` (8080),
  `CRONIFY_DB_PATH` (`./cronify.db`), `CRONIFY_TICK_INTERVAL_SECONDS` (60),
  `CRONIFY_DEFAULT_TIMEOUT_SECONDS` (30), `CRONIFY_DEFAULT_MAX_ATTEMPTS` (3),
  `CRONIFY_STALE_LOCK_TIMEOUT_SECONDS` (600), `CRONIFY_WEBHOOK_URL` (optional)
- CLI side: `CRONIFY_APP_URL`, `CRONIFY_SCHEDULER_URL`, `CRONIFY_ADMIN_TOKEN`
  (reused)

### License

MIT (see [`LICENSE`](LICENSE)).

### `withLock()`, concretely (`packages/cronify/src/lock.ts`, entry `cronify/lock`)

A standalone helper — decoupled from `defineJob()`/the scheduler — for
overlap protection when there's no self-hosted scheduler yet (e.g. pinging a
route with Vercel Cron or GitHub Actions).

```ts
interface LockStore {
  acquire(key: string, token: string, ttlSeconds: number): Promise<boolean>;
  release(key: string, token: string): Promise<void>;
}
```

`withLock(handler, { key, store, ttlSeconds = 300, onLocked = "skip" })`
generates a random token per invocation, acquires before running the
handler, releases in a `finally`. `release` only deletes if the stored value
still matches the token that call acquired (fencing) — a plain unconditional
delete would let a slow, still-running call release a different call's lock
after its own TTL expired and someone else reclaimed it. `onLocked: "skip"`
(default) silently no-ops when the lock is held; `"throw"` raises
`LockHeldError`. Either way a skip is invisible to the HTTP caller — a
skipped run and a real run both surface as `{"success":true}` from
`createRouteHandler` — so `withLock` also takes an `onSkip` callback,
confirmed by an e2e test (real concurrent requests against a real running
Next.js server) that a skip is otherwise indistinguishable from a run.

Ships two `LockStore` implementations: `createMemoryLockStore()`
(in-process, for local dev only — useless across separate serverless
invocations) and `createFileLockStore({ dir? })` (file-based, single
self-hosted machine, not safe across multiple machines). Redis/Vercel KV are
not bundled — deliberately, to avoid picking a client dependency — but plug
into the same two-method interface; documented with an example in
`packages/cronify/README.md`.

## `packages/cronify` — implementation notes

```
packages/cronify/
├── src/
│   ├── index.ts       defineJob() + validation — package entry "cronify"
│   ├── server.ts       createRouteHandler() — entry "cronify/server"
│   ├── lock.ts           withLock() + LockStore + memory/file stores — entry "cronify/lock"
│   ├── discover.ts     AST-based job discovery (never executes job files)
│   ├── generate.ts      writes route files + .cron-manifest.json, idempotent
│   ├── sync.ts           regenerate + POST to scheduler /api/v1/sync
│   ├── cli.ts             `cronify generate` / `cronify sync` bin
│   └── types.ts          shared types + defaults
└── test/                 vitest, one file per src module, 39 tests
```

Notable choices, in case they look surprising later:

- **No `next` dependency.** `createRouteHandler` returns plain
  `(request: Request) => Promise<Response>` handlers using the Fetch API
  directly (a hand-rolled `jsonResponse` helper, not `NextResponse.json`),
  since Next.js App Router route handlers accept standard Fetch types. Keeps
  the package framework-version-agnostic.
- **No glob/AST-execution dependency for discovery.** `discover.ts` walks
  `cron/` with plain `node:fs` recursion and parses each file with the
  `typescript` package's compiler API (a dependency, not just a peer — the
  CLI needs it to run standalone via `npx`). Deliberately rejected loading
  job files at runtime via something like `jiti`/`tsx`: executing arbitrary
  user files at codegen time would need their full dependency graph and env
  vars to resolve, and would run handler-adjacent module-scope code as a
  side effect of running `generate`. Static extraction avoids both.
- **`generate()` is destructive but scoped.** It deletes a job's generated
  route directory when that job disappears from `cron/`, but only if the
  existing file still contains the `AUTO-GENERATED by cronify` marker
  comment — it won't touch a directory a user repurposed by hand. Covered by
  `test/generate.test.ts`.
- **CLI parsing uses `node:util.parseArgs`**, not commander/yargs — the tool
  only has two subcommands and a handful of flags, and keeping this
  package's own dependency footprint small was treated as in keeping with
  the project's overall self-hosting-simplicity ethos (same reasoning SPEC.md
  gives for choosing Go for the scheduler).
- **`package.json` has a `typesVersions` field** mapping `server`/`lock` to
  their `.d.ts` files. Without it, `cronify/server` and `cronify/lock`
  type-check fine under `moduleResolution: "bundler"`/`"node16"`/`"nodenext"`
  (which read the `exports` map's `types` condition) but fail with "Cannot
  find module ... or its corresponding type declarations" under the legacy
  `"node"` algorithm, which predates `exports` and ignores it entirely.
  Found by actually running `next build` in a fresh app with no
  hand-authored `tsconfig.json`: Next.js's own auto-generated default is
  `moduleResolution: "node"`, so this wasn't a theoretical edge case — a
  brand-new project hits it immediately. `typesVersions` is the standard
  backward-compat mechanism for exactly this; verified fixed by rebuilding
  the same fixture after adding it.

Build/test commands (run from `packages/cronify/`):

```sh
npm install
npm run build       # tsup -> dist/ (esm + cjs + .d.ts)
npm test            # vitest, 39 tests passing as of this writing
npm run typecheck
```

## Explicitly out of scope for v1 (per SPEC.md)

No hosted/SaaS version, no multi-region scheduler, no workflow/step
orchestration (that's Inngest/Trigger.dev territory — this is scheduling +
retries + locking only), no auth provider for the dashboard beyond a shared
token.

## Next steps

Per the approved build order: the Go scheduler (tick loop, SQLite, the
`/api/v1/*` API above, retry/locking logic per the numbers above), then the
bundled dashboard, then Docker + one-click deploy buttons. Don't start
scheduler work without checking in with the user first.
