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

`sync` regenerates routes/manifest first, then POSTs the manifest to the
scheduler's `/api/v1/sync` endpoint, which reconciles jobs for this app
(creates new ones, updates changed ones, deletes ones no longer present).

Flags can also come from env vars: `CRONIFY_SCHEDULER_URL`, `CRONIFY_APP_URL`,
`CRONIFY_ADMIN_TOKEN`. `--source` (default: your `package.json` `"name"`)
groups jobs when one scheduler serves multiple apps.

## Package layout

- `defineJob()` / types — the `cronify` entry point (`src/index.ts`)
- `createRouteHandler()` — the `cronify/server` entry point (`src/server.ts`),
  used by generated route files
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
