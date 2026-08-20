# Cronify

Open-source, self-hostable scheduling for Next.js apps on serverless
platforms — retries and overlap protection on top of a plain cron ping.

[![Deploy to Render](https://render.com/images/deploy-to-render-button.svg)](https://render.com/deploy?repo=https://github.com/Ferinco/cronify)

- [`SPEC.md`](SPEC.md) — full project spec
- [`CLAUDE.md`](CLAUDE.md) — design decisions and current status, for anyone
  (human or AI) picking this repo back up
- [`packages/cronify`](packages/cronify) — the TypeScript package
  (`defineJob()`, route generation, CLI) — **implemented**
- [`scheduler`](scheduler) — the Go scheduler + dashboard —
  **implemented**, see its README for Docker/Railway/Fly.io/Render deploy
  instructions
- [`site`](site) — the marketing one-pager — **implemented**

License: MIT.
