import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { defineJob } from "../src/index.js";
import { createRouteHandler } from "../src/server.js";

const ORIGINAL_SECRET = process.env.CRON_SECRET;

beforeEach(() => {
  process.env.CRON_SECRET = "test-secret";
});

afterEach(() => {
  if (ORIGINAL_SECRET === undefined) {
    delete process.env.CRON_SECRET;
  } else {
    process.env.CRON_SECRET = ORIGINAL_SECRET;
  }
});

function request(headers: Record<string, string> = {}): Request {
  return new Request("https://example.com/api/cron/job", { headers });
}

describe("createRouteHandler", () => {
  it("returns 401 when the secret is missing or wrong", async () => {
    const job = defineJob({ id: "job", schedule: "0 9 * * *", handler: async () => {} });
    const { GET } = createRouteHandler(job);

    const res1 = await GET(request());
    expect(res1.status).toBe(401);

    const res2 = await GET(request({ authorization: "Bearer wrong" }));
    expect(res2.status).toBe(401);
  });

  it("runs the handler and returns success on a valid secret", async () => {
    const handler = vi.fn(async () => {});
    const job = defineJob({ id: "job", schedule: "0 9 * * *", handler });
    const { GET } = createRouteHandler(job);

    const res = await GET(request({ authorization: "Bearer test-secret" }));
    const body = await res.json();

    expect(res.status).toBe(200);
    expect(body.success).toBe(true);
    expect(typeof body.durationMs).toBe("number");
    expect(handler).toHaveBeenCalledOnce();
  });

  it("returns 500 with the error message when the handler throws", async () => {
    const job = defineJob({
      id: "job",
      schedule: "0 9 * * *",
      handler: async () => {
        throw new Error("boom");
      },
    });
    const { POST } = createRouteHandler(job);

    const res = await POST(request({ authorization: "Bearer test-secret" }));
    const body = await res.json();

    expect(res.status).toBe(500);
    expect(body.success).toBe(false);
    expect(body.error).toBe("boom");
  });

  it("returns 409 when the job is disabled", async () => {
    const job = defineJob({ id: "job", schedule: "0 9 * * *", enabled: false, handler: async () => {} });
    const { GET } = createRouteHandler(job);

    const res = await GET(request({ authorization: "Bearer test-secret" }));
    expect(res.status).toBe(409);
  });

  it("returns 500 when CRON_SECRET is not set", async () => {
    delete process.env.CRON_SECRET;
    const job = defineJob({ id: "job", schedule: "0 9 * * *", handler: async () => {} });
    const { GET } = createRouteHandler(job);

    const res = await GET(request({ authorization: "Bearer whatever" }));
    expect(res.status).toBe(500);
  });
});
