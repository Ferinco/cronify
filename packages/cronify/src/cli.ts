#!/usr/bin/env node
import { parseArgs } from "node:util";

import { generate } from "./generate.js";
import { sync } from "./sync.js";

function printHelp(): void {
  console.log(`cronify — scheduling for serverless Next.js apps

Usage:
  cronify generate              Discover jobs in cron/ and write routes + .cron-manifest.json
  cronify sync [options]        Regenerate, then register jobs with a running scheduler

Options for sync:
  --target <url>       Scheduler base URL (or CRONIFY_SCHEDULER_URL)
  --app-url <url>      Deployed app base URL (or CRONIFY_APP_URL)
  --token <token>      Scheduler admin token (or CRONIFY_ADMIN_TOKEN)
  --source <name>      Group name for these jobs (default: package.json "name")

  CRON_SECRET must be set in the environment — the same secret your deployed
  routes check. It isn't a CLI flag so it never appears in shell history/ps.

  -h, --help            Show this help
`);
}

async function main(): Promise<void> {
  const [command, ...rest] = process.argv.slice(2);

  if (!command || command === "-h" || command === "--help") {
    printHelp();
    process.exitCode = command ? 0 : 1;
    return;
  }

  if (command === "generate") {
    const result = generate();
    console.log(
      `cronify: wrote ${result.routesWritten.length} route(s), removed ${result.routesRemoved.length} stale route(s).`,
    );
    console.log(`cronify: manifest written to .cron-manifest.json (${result.manifest.jobs.length} job(s)).`);
    return;
  }

  if (command === "sync") {
    const { values } = parseArgs({
      args: rest,
      options: {
        target: { type: "string" },
        "app-url": { type: "string" },
        token: { type: "string" },
        source: { type: "string" },
      },
    });

    const target = values.target ?? process.env.CRONIFY_SCHEDULER_URL;
    const appUrl = values["app-url"] ?? process.env.CRONIFY_APP_URL;
    const token = values.token ?? process.env.CRONIFY_ADMIN_TOKEN;
    const cronSecret = process.env.CRON_SECRET;

    if (!target) throw new Error("cronify sync: missing --target (or set CRONIFY_SCHEDULER_URL)");
    if (!appUrl) throw new Error("cronify sync: missing --app-url (or set CRONIFY_APP_URL)");
    if (!token) throw new Error("cronify sync: missing --token (or set CRONIFY_ADMIN_TOKEN)");
    if (!cronSecret) throw new Error("cronify sync: missing CRON_SECRET (set it in your app's environment)");

    const result = await sync({ target, appUrl, token, cronSecret, source: values.source });
    console.log("cronify: sync complete.");
    console.log(JSON.stringify(result, null, 2));
    return;
  }

  console.error(`cronify: unknown command "${command}"`);
  printHelp();
  process.exitCode = 1;
}

main().catch((error: unknown) => {
  console.error(error instanceof Error ? error.message : error);
  process.exitCode = 1;
});
