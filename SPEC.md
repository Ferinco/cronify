# Cronify — Project Spec

## What it is

An open-source, self-hostable scheduling system built specifically for Next.js apps deployed on serverless platforms (Vercel, Netlify, etc.), where there's no persistent process to hold a clock. Solves the two things raw pingers (cron-job.org, GitHub Actions) don't: retries on failure, and overlap protection so a slow job never runs twice at once.

## The problem this solves

Serverless apps can't run a background process, so something external has to call your API route on a schedule. Free pingers exist (cron-job.org, GitHub Actions) and are reliable for the basic "ping a URL" job — but they don't retry failed requests and don't prevent the same job firing again if the previous run is still in progress. This project is the layer that adds retries + locking + a dashboard, self-hosted so nobody has to trust a third party or pay for Vercel Pro just for cron frequency.

## Four pieces

### 1. The package (`defineJob()`)

Installed in the user's Next.js app. Written in **TypeScript** — has to be, since it runs inside a Next.js app's build process and generates a Next.js route file.

```ts
import { defineJob } from "cronify";

export default defineJob({
  id: "daily-report",
  schedule: "0 9 * * *",
  handler: async () => {
    /* their logic */
  },
});
```

- Auto-generates the API route (`/api/cron/[jobId]`) that verifies a shared secret, runs the handler, returns success/failure
- Emits a build-time manifest (`.cron-manifest.json`) listing all jobs + schedules — this is what gets registered with a scheduler target
- CLI command `npx cronify sync --target <scheduler-url>` registers/updates jobs against a running scheduler instance

### 2. The scheduler (self-hosted by each user, on their own infra)

Written in **Go** — deliberately, not TypeScript/Node. Rationale: this piece's entire value proposition is "dead simple to self-host." A single static Go binary means no `node_modules`, no runtime version mismatches, a much smaller Docker image, and lower resource usage on whatever cheap box someone deploys it to. This is the one piece where the language choice directly reinforces the product's core pitch. (Decision confirmed — Node/TypeScript was considered as a faster-to-ship alternative given solo/parallel-project bandwidth, but Go was chosen deliberately for the stronger long-term product fit.)

- Single always-on process holding its own tick loop (this is the one piece in the whole project that CAN hold its own clock, since it's not serverless)
- SQLite for storage — job configs, run history, no need for Postgres at this scale
- Tick loop: every minute, check due jobs, fire an HTTP POST to the target URL with the shared secret header, record result
- **Retry with backoff** on failure — exact policy TBD, propose before building (see Open Questions)
- **Overlap protection**: before firing, check if the previous run of that job is still marked in-progress — skip or queue instead of double-firing. Needs a stale-lock timeout in case a run gets stuck "in progress" after a crash.
- Small HTTP API for the package's `sync` command and dashboard to talk to

### 3. The dashboard (bundled into the scheduler's Go binary)

- Served from the same process as the scheduler — one binary, one Docker image, one deploy
- Server-rendered HTML (Go's `html/template` or similar), not a separate JS framework — keeps the binary small and dependency-free
- Shows: registered jobs, next run time, last run status, run history/logs per job
- Manual "run now" and "pause" buttons
- Optional failure alerting via a webhook URL (Slack/Discord) the user provides

### 4. The marketing site (one-pager, separate from the product)

Written in **Next.js** — this is the one piece where Next.js is genuinely the right tool: static, content-focused, SEO-friendly, no background-process mismatch like the scheduler had.

- Single page: headline explaining what it is and who it's for, a `defineJob()` code snippet, a short "why" section (Vercel Hobby limits, cron-job.org doesn't retry/lock), a deploy button, a link to the repo
- No pricing, no testimonials, no multi-page structure — deliberately minimal
- Deployed to Vercel

## Tech stack summary

| Piece                   | Language/stack                   | Why                                               |
| ----------------------- | -------------------------------- | ------------------------------------------------- |
| Package (`defineJob()`) | TypeScript                       | Runs inside Next.js build process                 |
| Scheduler + dashboard   | Go, SQLite, server-rendered HTML | Self-hosting simplicity is the core pitch         |
| Marketing site          | Next.js                          | Static/content-focused, Next.js's actual strength |

Packaging: Docker for the scheduler service, with one-click deploy buttons for Railway / Fly.io / Render.

## Repo structure

```
cronify/
├── packages/
│   └── cron/           ← TypeScript, the defineJob() package
├── scheduler/           ← Go, the tick loop + HTTP API + dashboard
├── site/                ← Next.js, the marketing one-pager
├── SPEC.md
```

Polyglot repo, not a single-language npm workspace — the package and site are independently buildable from the scheduler.

## Build order (do in this sequence, each step independently useful)

1. **`defineJob()` + route generation** — package alone, no scheduler needed yet. Users can already point Vercel Cron or GitHub Actions at the generated route manually. Shippable v0.1.
2. **`withLock()` primitive** — a standalone helper (pluggable store: Redis, file, or Vercel KV) users can wrap their handler in for overlap protection, even without the full scheduler.
3. **The self-hosted scheduler** — the tick loop, SQLite storage, retry logic, overlap protection, HTTP API.
4. **The dashboard** — bundled UI on top of the scheduler once it's stable.
5. **Docker packaging + one-click deploy buttons** — this is a real part of the product experience, not an afterthought.

## Open technical questions — propose concrete answers before writing code

- **Manifest format**: exact JSON shape of `.cron-manifest.json` (fields per job)
- **API contract**: exact endpoints the Go scheduler exposes (e.g. `POST /jobs`, `GET /jobs`, `DELETE /jobs/:id`) and what `sync` sends/expects
- **Retry policy, numerically**: how many retries, what backoff strategy (fixed vs exponential), what counts as failure (non-2xx? timeout after how long?)
- **Locking mechanism, concretely**: schema for tracking run status, stale-lock timeout duration
- **Secret/auth mechanism**: header name, exact match vs HMAC signature of payload
- **Next.js routing convention**: App Router (assume this unless told otherwise)
- **Env var naming convention**: e.g. `CRON_SECRET`, `CRON_SCHEDULER_URL`
- **License**: MIT (default assumption for this kind of open-source dev tool, confirm before publishing)

## Scope discipline — explicitly SKIP for v1

- No hosted/SaaS version — self-hosted only, nobody depends on your uptime
- No multi-region scheduler support
- No complex workflow/step orchestration (that's Inngest/Trigger.dev territory) — this is just scheduling + retries + locking, nothing more
- No built-in auth provider for the dashboard beyond simple username/password or a shared secret
