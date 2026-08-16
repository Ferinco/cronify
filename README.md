# Cronify

Open-source, self-hostable scheduling for Next.js apps on serverless
platforms — retries and overlap protection on top of a plain cron ping.

- [`SPEC.md`](SPEC.md) — full project spec
- [`CLAUDE.md`](CLAUDE.md) — design decisions and current status, for anyone
  (human or AI) picking this repo back up
- [`packages/cronify`](packages/cronify) — the TypeScript package
  (`defineJob()`, route generation, CLI) — **implemented**
- [`scheduler`](scheduler) — the Go scheduler + dashboard — not yet built
- [`site`](site) — the marketing one-pager — not yet built

License: MIT.
