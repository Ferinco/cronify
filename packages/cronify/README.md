# cronify

Scheduling for serverless Next.js apps. Define jobs in TypeScript, let cronify
generate the API routes, and register them with a self-hosted [cronify
scheduler](../../scheduler) that adds retries and overlap protection on top of
a plain cron ping.

This package is the piece that runs inside your Next.js app. See
[`../../SPEC.md`](../../SPEC.md) for the full project design.

## 1. Define a job

Jobs live in a `cron/` directory at your project root. Each file's default
export must be `defineJob(...)` called directly — cronify reads `id` and
`schedule` via static analysis at build time, so they must be literal values,
not computed expressions.

```ts
// cron/daily-report.ts
import { defineJob } from "cronify";

export default defineJob({
  id: "daily-report",
  schedule: "0 9 * * *",
  handler: async () => {
    // your logic
  },
});
```

Optional fields: `timeoutSeconds` (default 30), `maxAttempts` (default 3),
`enabled` (default true), `description`.

## 2. Generate routes

```sh
npx cronify generate
```

For each job this writes:

- `app/api/cron/<id>/route.ts` (or `src/app/...` if you use a `src/` layout)
  — a small generated file that wires your job into `GET`/`POST` handlers.
  Don't edit these; rerunning `generate` overwrites them, and removes routes
  for jobs that no longer exist.
- `.cron-manifest.json` at the project root — the full list of jobs and their
  schedules, consumed by `cronify sync`.

Set `CRON_SECRET` in your Next.js app's environment. Generated routes reject
any request whose `Authorization: Bearer <token>` header doesn't match it.

Without a scheduler, you can already point Vercel Cron or GitHub Actions at
these routes manually — the scheduler adds retries and overlap protection on
top, but isn't required to ship v0.1.

## 3. Sync with a scheduler

```sh
npx cronify sync \
  --target https://scheduler.example.com \
  --app-url https://myapp.vercel.app \
  --token $CRONIFY_ADMIN_TOKEN
```

`sync` regenerates routes/manifest first, then POSTs the manifest — plus
`CRON_SECRET` from your environment, so the scheduler knows what to send back
when it fires your routes — to the scheduler's `/api/v1/sync` endpoint, which
reconciles jobs for this app (creates new ones, updates changed ones, deletes
ones no longer present).

Flags can also come from env vars: `CRONIFY_SCHEDULER_URL`, `CRONIFY_APP_URL`,
`CRONIFY_ADMIN_TOKEN`. `--source` (default: your `package.json` `"name"`)
groups jobs when one scheduler serves multiple apps. `CRON_SECRET` has no
flag equivalent — it's read from the environment only, so it never appears
in shell history or `ps` output.

## `withLock()` — overlap protection without the scheduler

Wrap a handler to stop two invocations from running at once, even if you're
pinging the route with Vercel Cron or GitHub Actions instead of the cronify
scheduler:

```ts
import { defineJob } from "cronify";
import { withLock, createFileLockStore } from "cronify/lock";

const store = createFileLockStore(); // single self-hosted instance

export default defineJob({
  id: "daily-report",
  schedule: "0 9 * * *",
  handler: withLock(
    async () => {
      // your logic
    },
    {
      key: "daily-report",
      store,
      onSkip: () => console.log('cronify: "daily-report" skipped, previous run still in progress'),
    },
  ),
});
```

A skipped run still resolves with `{"success":true}` from the route — from the HTTP response alone there's no way to tell "ran" from "skipped." `onSkip` is the way to make that visible in your platform's logs (Vercel/GitHub Actions/wherever), which is the only observability you have without the full scheduler.

Built-in stores:

- `createMemoryLockStore()` — in-process only. Fine for local dev/tests, not
  useful across separate serverless invocations.
- `createFileLockStore({ dir? })` — file-based, for a single self-hosted
  machine without Redis. Not safe across multiple machines.

For Redis, Vercel KV, or anything else, implement the two-method `LockStore`
interface yourself:

```ts
interface LockStore {
  acquire(key: string, token: string, ttlSeconds: number): Promise<boolean>;
  release(key: string, token: string): Promise<void>;
}
```

`acquire` must be an atomic set-if-absent-or-expired with a TTL (e.g. Redis
`SET key token NX PX ttlMs`). `release` must only delete the key if its
current value equals `token` (e.g. a Lua script doing `GET` then `DEL`) — a
plain unconditional delete would let a slow, still-running call release a
different call's lock after its own lock expired.

`withLock` options: `ttlSeconds` (default 300), `onLocked: "skip" | "throw"`
(default `"skip"` — silently returns without running the handler; `"throw"`
raises `LockHeldError`), `onSkip` (called whenever a run is skipped, in
either `onLocked` mode).

## Package layout

- `defineJob()` / types — the `cronify` entry point (`src/index.ts`)
- `createRouteHandler()` — the `cronify/server` entry point (`src/server.ts`),
  used by generated route files
- `withLock()` / lock stores — the `cronify/lock` entry point (`src/lock.ts`)
- `discoverJobs()` — AST-based job discovery, never executes your files
  (`src/discover.ts`)
- `generate()` — writes routes + manifest (`src/generate.ts`)
- `sync()` — regenerate + POST to a scheduler (`src/sync.ts`)
- `cli.ts` — the `cronify` bin

## Development

```sh
npm install
npm run build      # tsup -> dist/
npm test           # vitest
npm run typecheck
```
