import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { sync } from "../src/sync.js";

let dir: string;
let fetchSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  dir = mkdtempSync(join(tmpdir(), "cronify-sync-"));
  mkdirSync(join(dir, "app"), { recursive: true });
  mkdirSync(join(dir, "cron"), { recursive: true });
  writeFileSync(
    join(dir, "cron", "job.ts"),
    `import { defineJob } from "cronify";
export default defineJob({ id: "job", schedule: "0 9 * * *", handler: async () => {} });
`,
  );
  writeFileSync(join(dir, "package.json"), JSON.stringify({ name: "my-app" }));
});

afterEach(() => {
  rmSync(dir, { recursive: true, force: true });
  fetchSpy?.mockRestore();
});

describe("sync", () => {
  it("regenerates, then POSTs the manifest with source derived from package.json", async () => {
    fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ created: ["job"], updated: [], deleted: [], unchanged: [] }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    const result = await sync({
      target: "https://scheduler.example.com",
      appUrl: "https://myapp.vercel.app",
      token: "admin-token",
      root: dir,
    });

    expect(fetchSpy).toHaveBeenCalledOnce();
    const [url, init] = fetchSpy.mock.calls[0];
    expect(url).toBe("https://scheduler.example.com/api/v1/sync");
    expect(init?.headers).toMatchObject({ authorization: "Bearer admin-token" });

    const body = JSON.parse(init?.body as string);
    expect(body.source).toBe("my-app");
    expect(body.appUrl).toBe("https://myapp.vercel.app");
    expect(body.jobs).toHaveLength(1);
    expect(body.jobs[0].id).toBe("job");

    expect(result).toEqual({ created: ["job"], updated: [], deleted: [], unchanged: [] });
  });

  it("uses an explicit --source over package.json", async () => {
    fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("{}", { status: 200 }));

    await sync({
      target: "https://scheduler.example.com",
      appUrl: "https://myapp.vercel.app",
      token: "admin-token",
      source: "explicit-source",
      root: dir,
    });

    const [, init] = fetchSpy.mock.calls[0];
    const body = JSON.parse(init?.body as string);
    expect(body.source).toBe("explicit-source");
  });

  it("throws a clear error when the scheduler is unreachable", async () => {
    fetchSpy = vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("ECONNREFUSED"));

    await expect(
      sync({ target: "https://scheduler.example.com", appUrl: "https://myapp.vercel.app", token: "t", root: dir }),
    ).rejects.toThrow(/couldn't reach scheduler/);
  });

  it("throws when the scheduler responds with a non-2xx status", async () => {
    fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(new Response("unauthorized", { status: 401, statusText: "Unauthorized" }));

    await expect(
      sync({ target: "https://scheduler.example.com", appUrl: "https://myapp.vercel.app", token: "bad", root: dir }),
    ).rejects.toThrow(/401/);
  });
});
