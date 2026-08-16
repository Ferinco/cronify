import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";

import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { DiscoveryError, discoverJobs } from "../src/discover.js";

let dir: string;

beforeEach(() => {
  dir = mkdtempSync(join(tmpdir(), "cronify-discover-"));
});

afterEach(() => {
  rmSync(dir, { recursive: true, force: true });
});

function write(relativePath: string, contents: string): void {
  const full = join(dir, relativePath);
  mkdirSync(dirname(full), { recursive: true });
  writeFileSync(full, contents, "utf8");
}

describe("discoverJobs", () => {
  it("extracts id and schedule from a valid job file", () => {
    write(
      "cron/daily-report.ts",
      `import { defineJob } from "cronify";
export default defineJob({
  id: "daily-report",
  schedule: "0 9 * * *",
  handler: async () => {},
});
`,
    );

    const jobs = discoverJobs(join(dir, "cron"));

    expect(jobs).toHaveLength(1);
    expect(jobs[0].id).toBe("daily-report");
    expect(jobs[0].route).toBe("/api/cron/daily-report");
    expect(jobs[0].timeoutSeconds).toBe(30);
    expect(jobs[0].maxAttempts).toBe(3);
    expect(jobs[0].enabled).toBe(true);
    expect(jobs[0].description).toBeNull();
  });

  it("applies overrides for timeoutSeconds, maxAttempts, enabled, description", () => {
    write(
      "cron/custom.ts",
      `import { defineJob } from "cronify";
export default defineJob({
  id: "custom",
  schedule: "*/5 * * * *",
  timeoutSeconds: 60,
  maxAttempts: 5,
  enabled: false,
  description: "does a thing",
  handler: async () => {},
});
`,
    );

    const [job] = discoverJobs(join(dir, "cron"));

    expect(job.timeoutSeconds).toBe(60);
    expect(job.maxAttempts).toBe(5);
    expect(job.enabled).toBe(false);
    expect(job.description).toBe("does a thing");
  });

  it("throws on duplicate ids", () => {
    write(
      "cron/a.ts",
      `import { defineJob } from "cronify";
export default defineJob({ id: "dup", schedule: "0 9 * * *", handler: () => {} });
`,
    );
    write(
      "cron/b.ts",
      `import { defineJob } from "cronify";
export default defineJob({ id: "dup", schedule: "0 9 * * *", handler: () => {} });
`,
    );

    expect(() => discoverJobs(join(dir, "cron"))).toThrow(DiscoveryError);
  });

  it("throws when id is a non-literal expression", () => {
    write(
      "cron/dynamic.ts",
      `import { defineJob } from "cronify";
const id = "dynamic-" + Date.now();
export default defineJob({ id, schedule: "0 9 * * *", handler: () => {} });
`,
    );

    expect(() => discoverJobs(join(dir, "cron"))).toThrow(DiscoveryError);
  });

  it("throws when defineJob isn't the default export", () => {
    write(
      "cron/named.ts",
      `import { defineJob } from "cronify";
export const job = defineJob({ id: "named", schedule: "0 9 * * *", handler: () => {} });
`,
    );

    expect(() => discoverJobs(join(dir, "cron"))).toThrow(DiscoveryError);
  });

  it("ignores files that don't call defineJob", () => {
    write("cron/helpers.ts", `export function util(): number { return 1; }\n`);

    const jobs = discoverJobs(join(dir, "cron"));

    expect(jobs).toHaveLength(0);
  });

  it("sorts jobs by id", () => {
    write(
      "cron/zeta.ts",
      `import { defineJob } from "cronify";
export default defineJob({ id: "zeta", schedule: "0 9 * * *", handler: () => {} });
`,
    );
    write(
      "cron/alpha.ts",
      `import { defineJob } from "cronify";
export default defineJob({ id: "alpha", schedule: "0 9 * * *", handler: () => {} });
`,
    );

    const jobs = discoverJobs(join(dir, "cron"));

    expect(jobs.map((j) => j.id)).toEqual(["alpha", "zeta"]);
  });
});
