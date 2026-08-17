import Link from "next/link";

const REPO_URL = "https://github.com/Ferinco/cronify";
const SELF_HOST_URL = `${REPO_URL}/blob/main/scheduler/README.md`;

export default function Home() {
  return (
    <div className="page">
      <header className="site-header">
        <Link href="/" className="wordmark mono">
          cronify
        </Link>
        <a href={REPO_URL} className="header-link" target="_blank" rel="noreferrer">
          GitHub ↗
        </a>
      </header>

      <section className="hero">
        <h1>Cron jobs that actually retry.</h1>
        <p>
          Cronify adds retries and overlap protection to scheduled jobs on serverless Next.js apps — self-hosted, so
          you&apos;re not depending on a third party&apos;s uptime or a free tier&apos;s limits.
        </p>
      </section>

      <div className="code-block">
        <div className="code-block-label mono">cron/daily-report.ts</div>
        <pre className="mono">
          <code>{`import { defineJob } from "cronify";

export default defineJob({
  id: "daily-report",
  schedule: "0 9 * * *",
  handler: async () => {
    /* your logic */
  },
});`}</code>
        </pre>
      </div>

      <section className="why">
        <h2>Why not just use...</h2>
        <div className="why-grid">
          <div className="why-card">
            <h3>Vercel Cron?</h3>
            <p>
              The Hobby plan caps you at one cron job, once a day. Cronify runs on infrastructure you control, so
              schedule as many jobs as you want, as often as you want.
            </p>
          </div>
          <div className="why-card">
            <h3>Free pingers?</h3>
            <p>
              cron-job.org and GitHub Actions can hit a URL on schedule, but if the request fails or the previous run
              is still going, they don&apos;t do anything about it. Cronify retries with backoff and stops overlapping
              runs.
            </p>
          </div>
        </div>
      </section>

      <div className="cta-row">
        <a className="btn btn-primary" href={SELF_HOST_URL} target="_blank" rel="noreferrer">
          Self-host it →
        </a>
        <a className="btn btn-secondary" href={REPO_URL} target="_blank" rel="noreferrer">
          View on GitHub
        </a>
      </div>

      <footer className="site-footer">
        <span>MIT licensed.</span>
        <a href={REPO_URL} target="_blank" rel="noreferrer">
          github.com/Ferinco/cronify
        </a>
      </footer>
    </div>
  );
}
