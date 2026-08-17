# site

The marketing one-pager, per [`../SPEC.md`](../SPEC.md) piece 4: headline, a
`defineJob()` snippet, a short "why" section, a CTA row, and a link to the
repo. No pricing, no testimonials, no multi-page structure — deliberately
minimal. Plain Next.js App Router + hand-written CSS, no Tailwind or
component library, consistent with the rest of the project's small-footprint
approach (see [`../CLAUDE.md`](../CLAUDE.md)).

The "Self-host it" button currently links to
[`scheduler/README.md`](../scheduler/README.md) — real, working self-host
instructions today. It's meant to be swapped for a one-click Railway/Fly.io/
Render deploy link once step 5 (Docker packaging) exists.

## Develop

```sh
npm install
npm run dev     # http://localhost:3000
```

## Build

```sh
npm run build
```

## Deploy

Not deployed by anything in this repo. Per SPEC.md this is meant to be
deployed to Vercel — connect the repo (with `site/` as the project root) via
the Vercel dashboard, or run `npx vercel` from within `site/`.
