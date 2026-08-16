import { timingSafeEqual } from "node:crypto";

import type { JobDefinition } from "./types.js";

export type RouteHandler = (request: Request) => Promise<Response>;

function jsonResponse(body: unknown, status: number): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

function safeEqual(a: string, b: string): boolean {
  const bufA = Buffer.from(a);
  const bufB = Buffer.from(b);
  if (bufA.length !== bufB.length) return false;
  return timingSafeEqual(bufA, bufB);
}

/**
 * Builds the GET/POST handlers for a job's Next.js route. Verifies the
 * CRON_SECRET bearer token, runs the handler, and reports success/failure
 * as JSON. Retries and overlap protection live in the scheduler, not here —
 * this route just needs to be a faithful, single-attempt trigger.
 */
export function createRouteHandler(job: JobDefinition): { GET: RouteHandler; POST: RouteHandler } {
  const handler: RouteHandler = async (request) => {
    const secret = process.env.CRON_SECRET;
    if (!secret) {
      console.error(`cronify: CRON_SECRET is not set; refusing all requests for job "${job.id}".`);
      return jsonResponse({ success: false, error: "Server misconfigured" }, 500);
    }

    const authHeader = request.headers.get("authorization") ?? "";
    if (!safeEqual(authHeader, `Bearer ${secret}`)) {
      return jsonResponse({ success: false, error: "Unauthorized" }, 401);
    }

    if (job.enabled === false) {
      return jsonResponse({ success: false, error: "Job is disabled" }, 409);
    }

    const startedAt = Date.now();
    try {
      await job.handler();
      return jsonResponse({ success: true, durationMs: Date.now() - startedAt }, 200);
    } catch (error) {
      console.error(`cronify: job "${job.id}" failed:`, error);
      return jsonResponse(
        { success: false, error: error instanceof Error ? error.message : String(error) },
        500,
      );
    }
  };

  return { GET: handler, POST: handler };
}
