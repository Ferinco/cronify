import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { generate } from "../src/generate.js";

let dir: string;

beforeEach(() => {
  dir = mkdtempSync(join(tmpdir(), "cronify-generate-"));
  mkdirSync(join(dir, "app"), { recursive: true });
  mkdirSync(join(dir, "cron"), { recursive: true });
});

afterEach(() => {
  rmSync(dir, { recursive: true, force: true });
});

describe("generate", () => {
  it("writes a manifest and route file for each discovered job", () => {
    writeFileSync(
      join(dir, "cron", "daily-report.ts"),
      `import { defineJob } from "cronify";
export default defineJob({ id: "daily-report", schedule: "0 9 * * *", handler: async () => {} });
`,
    );

    const result = generate(dir);

    expect(result.manifest.jobs).toHaveLength(1);
    expect(result.routesWritten).toHaveLength(1);

    const manifestOnDisk = JSON.parse(readFileSync(join(dir, ".cron-manifest.json"), "utf8"));
    expect(manifestOnDisk.jobs[0].id).toBe("daily-report");
    expect(manifestOnDisk.jobs[0].route).toBe("/api/cron/daily-report");

    const routeContents = readFileSync(join(dir, "app", "api", "cron", "daily-report", "route.ts"), "utf8");
    expect(routeContents).toContain("AUTO-GENERATED");
    expect(routeContents).toContain('import job from "../../../../cron/daily-report"');
    expect(routeContents).toContain('import { createRouteHandler } from "cronify/server"');
    expect(routeContents).toContain("export const { GET, POST } = createRouteHandler(job)");
  });

  it("removes stale generated routes for jobs that disappear", () => {
    writeFileSync(
      join(dir, "cron", "a.ts"),
      `import { defineJob } from "cronify";
export default defineJob({ id: "a", schedule: "0 9 * * *", handler: async () => {} });
`,
    );
    generate(dir);
    expect(existsSync(join(dir, "app", "api", "cron", "a", "route.ts"))).toBe(true);

    rmSync(join(dir, "cron", "a.ts"));
    writeFileSync(
      join(dir, "cron", "b.ts"),
      `import { defineJob } from "cronify";
export default defineJob({ id: "b", schedule: "0 9 * * *", handler: async () => {} });
`,
    );

    const result = generate(dir);

    expect(existsSync(join(dir, "app", "api", "cron", "a", "route.ts"))).toBe(false);
    expect(existsSync(join(dir, "app", "api", "cron", "b", "route.ts"))).toBe(true);
    expect(result.routesRemoved).toHaveLength(1);
  });

  it("does not remove a route directory it didn't generate", () => {
    mkdirSync(join(dir, "app", "api", "cron", "manual"), { recursive: true });
    writeFileSync(join(dir, "app", "api", "cron", "manual", "route.ts"), "// hand-written route\n");
    writeFileSync(
      join(dir, ".cron-manifest.json"),
      JSON.stringify({
        version: 1,
        generatedAt: new Date().toISOString(),
        jobs: [
          {
            id: "manual",
            schedule: "0 9 * * *",
            route: "/api/cron/manual",
            enabled: true,
            timeoutSeconds: 30,
            maxAttempts: 3,
            description: null,
          },
        ],
      }),
    );

    generate(dir);

    expect(existsSync(join(dir, "app", "api", "cron", "manual", "route.ts"))).toBe(true);
  });

  it("supports src/app as well as app/", () => {
    rmSync(join(dir, "app"), { recursive: true, force: true });
    mkdirSync(join(dir, "src", "app"), { recursive: true });
    writeFileSync(
      join(dir, "cron", "job.ts"),
      `import { defineJob } from "cronify";
export default defineJob({ id: "job", schedule: "0 9 * * *", handler: async () => {} });
`,
    );

    generate(dir);

    expect(existsSync(join(dir, "src", "app", "api", "cron", "job", "route.ts"))).toBe(true);
  });

  it("throws if no app directory exists", () => {
    rmSync(join(dir, "app"), { recursive: true, force: true });
    expect(() => generate(dir)).toThrow(/App Router/);
  });

  it("throws if no cron directory exists", () => {
    rmSync(join(dir, "cron"), { recursive: true, force: true });
    expect(() => generate(dir)).toThrow(/cron/);
  });
});
