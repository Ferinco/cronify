# scheduler (not yet implemented)

Will hold the Go tick loop, SQLite storage, retry/backoff, overlap-protection
locking, the `/api/v1/*` HTTP API, and the bundled server-rendered dashboard.
See [`../SPEC.md`](../SPEC.md) and [`../CLAUDE.md`](../CLAUDE.md) for the
design this is built against, including the approved API contract, retry
policy numbers, and locking schema.

Build order (per `SPEC.md`): scheduler comes after `packages/cronify` is
stable, dashboard after that.
