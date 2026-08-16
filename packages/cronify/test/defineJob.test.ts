import { describe, expect, it } from "vitest";

import { defineJob } from "../src/index.js";

describe("defineJob", () => {
  it("returns a job definition for valid input", () => {
    const job = defineJob({ id: "daily-report", schedule: "0 9 * * *", handler: async () => {} });
    expect(job.id).toBe("daily-report");
    expect(job.__cronifyJob).toBe(true);
  });

  it("rejects an invalid id", () => {
    expect(() => defineJob({ id: "bad id!", schedule: "0 9 * * *", handler: () => {} })).toThrow(/id/);
  });

  it("rejects a schedule that isn't 5 fields", () => {
    expect(() => defineJob({ id: "job", schedule: "* * *", handler: () => {} })).toThrow(/schedule/);
  });

  it("rejects a missing handler", () => {
    expect(() =>
      // @ts-expect-error testing runtime validation of a missing handler
      defineJob({ id: "job", schedule: "0 9 * * *" }),
    ).toThrow(/handler/);
  });
});
