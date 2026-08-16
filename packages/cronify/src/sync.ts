import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

import { generate } from "./generate.js";

export interface SyncOptions {
  target: string;
  appUrl: string;
  token: string;
  cronSecret: string;
  source?: string;
  root?: string;
}

function resolveSource(root: string, explicit?: string): string {
  if (explicit) return explicit;
  const pkgPath = join(root, "package.json");
  if (existsSync(pkgPath)) {
    try {
      const pkg = JSON.parse(readFileSync(pkgPath, "utf8")) as { name?: string };
      if (typeof pkg.name === "string" && pkg.name) return pkg.name;
    } catch {
      // fall through to the error below
    }
  }
  throw new Error(
    `cronify: couldn't determine a "source" name (no package.json "name" field found at ${root}). ` +
      `Pass --source explicitly.`,
  );
}

export async function sync(options: SyncOptions): Promise<unknown> {
  const root = options.root ?? process.cwd();
  const { manifest } = generate(root);
  const source = resolveSource(root, options.source);

  const url = new URL("/api/v1/sync", options.target).toString();
  const body = JSON.stringify({
    source,
    appUrl: options.appUrl,
    cronSecret: options.cronSecret,
    jobs: manifest.jobs,
  });

  let response: Response;
  try {
    response = await fetch(url, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        authorization: `Bearer ${options.token}`,
      },
      body,
    });
  } catch (error) {
    throw new Error(
      `cronify: couldn't reach scheduler at ${url}: ${error instanceof Error ? error.message : String(error)}`,
    );
  }

  const text = await response.text();
  let parsed: unknown = text;
  try {
    parsed = text ? JSON.parse(text) : undefined;
  } catch {
    // leave parsed as raw text
  }

  if (!response.ok) {
    throw new Error(`cronify: scheduler responded ${response.status} ${response.statusText}: ${text}`);
  }

  return parsed;
}
