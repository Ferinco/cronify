import Image from "next/image";
import Link from "next/link";
import { HeroIllustrations } from "./HeroIllustrations";

const REPO_URL = "https://github.com/Ferinco/cronify";
const SELF_HOST_URL = `${REPO_URL}/blob/main/scheduler/README.md`;

export default function Home() {
  return (
    <div className="backdrop">
      <HeroIllustrations />
      <div className="page">
        <header className="site-header">
          <Link href="/" className="wordmark">
            <Image
              src="/images/cronify-logo-white.png"
              alt="cronify"
              width={99}
              height={33}
              priority
            />
          </Link>
          <div className="header-actions">
            <a
              href={REPO_URL}
              className="btn btn-secondary"
              target="_blank"
              rel="noreferrer"
            >
              GitHub
            </a>
            <a
              href={SELF_HOST_URL}
              className="btn btn-primary"
              target="_blank"
              rel="noreferrer"
            >
              Self-host it
            </a>
          </div>
        </header>

        <section className="hero mt-10">
          <h1>Cron jobs that actually retry.</h1>
          <p>
            Cronify adds retries and overlap protection to scheduled jobs on
            serverless Next.js apps — self-hosted, so you&apos;re not depending
            on a third party&apos;s uptime or a free tier&apos;s limits.
          </p>

          <div className="cta-row">
            <a
              className="btn btn-primary"
              href={SELF_HOST_URL}
              target="_blank"
              rel="noreferrer"
            >
              Self-host it →
            </a>
            <a
              className="btn btn-secondary"
              href={REPO_URL}
              target="_blank"
              rel="noreferrer"
            >
              View on GitHub
            </a>
          </div>
        </section>

        <div className="code-window">
          <div className="code-window-bar">
            <div className="code-window-dots">
              <span />
              <span />
              <span />
            </div>
            <span className="code-window-label mono">cron/daily-report.ts</span>
          </div>
          <pre className="mono">
            <code>
              <span className="tok-kw">import</span> {"{ "}
              <span className="tok-fn">defineJob</span>
              {" }"} <span className="tok-kw">from</span>{" "}
              <span className="tok-str">&quot;cronify&quot;</span>;{"\n\n"}
              <span className="tok-kw">export default</span>{" "}
              <span className="tok-fn">defineJob</span>({"{"}
              {"\n  "}
              <span className="tok-prop">id</span>:{" "}
              <span className="tok-str">&quot;daily-report&quot;</span>,{"\n  "}
              <span className="tok-prop">schedule</span>:{" "}
              <span className="tok-str">&quot;0 9 * * *&quot;</span>,{"\n  "}
              <span className="tok-prop">handler</span>:{" "}
              <span className="tok-kw">async</span> () {"=>"} {"{"}
              {"\n    "}
              <span className="tok-com">{"/* your logic */"}</span>
              {"\n  "}
              {"},"}
              {"\n"}
              {"});"}
            </code>
          </pre>
        </div>

        <p className="compat">
          Works with <strong>Next.js App Router</strong> · deployed on{" "}
          <strong>Vercel</strong>, <strong>Netlify</strong>, or anywhere
          serverless
        </p>

        <section className="why">
          <h2>Why not just use...</h2>
          <div className="why-grid">
            <div className="why-card">
              <h3>Vercel Cron?</h3>
              <p>
                The Hobby plan caps you at one cron job, once a day. Cronify
                runs on infrastructure you control, so schedule as many jobs as
                you want, as often as you want.
              </p>
            </div>
            <div className="why-card">
              <h3>Free pingers?</h3>
              <p>
                cron-job.org and GitHub Actions can hit a URL on schedule, but
                if the request fails or the previous run is still going, they
                don&apos;t do anything about it. Cronify retries with backoff
                and stops overlapping runs.
              </p>
            </div>
          </div>
        </section>

        <footer className="site-footer">
          <span>MIT licensed.</span>
          <a href={REPO_URL} target="_blank" rel="noreferrer">
            github.com/Ferinco/cronify
          </a>
        </footer>
      </div>
    </div>
  );
}
