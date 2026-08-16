import type { DefineJobOptions, JobDefinition } from "./types.js";

export type {
  DefineJobOptions,
  JobDefinition,
  Manifest,
  ManifestJobEntry,
} from "./types.js";

const ID_PATTERN = /^[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?$/;
const CRON_FIELD_COUNT = 5;

/**
 * Declares a cron job. `id` and `schedule` must be static string literals —
 * cronify's CLI reads them via static analysis at build time and never
 * executes this file, so it never needs your handler's runtime dependencies
 * or env vars just to generate routes and the manifest.
 */
export function defineJob(options: DefineJobOptions): JobDefinition {
  if (!options.id || !ID_PATTERN.test(options.id)) {
    throw new Error(
      `defineJob: "id" must be a non-empty string of letters, numbers, and hyphens ` +
        `(got ${JSON.stringify(options.id)}). It becomes the route segment /api/cron/${options.id}.`,
    );
  }

  const fieldCount = options.schedule?.trim().split(/\s+/).filter(Boolean).length ?? 0;
  if (fieldCount !== CRON_FIELD_COUNT) {
    throw new Error(
      `defineJob "${options.id}": "schedule" must be a standard 5-field cron expression ` +
        `(minute hour day month weekday), got ${JSON.stringify(options.schedule)}.`,
    );
  }

  if (typeof options.handler !== "function") {
    throw new Error(`defineJob "${options.id}": "handler" must be a function.`);
  }

  return { ...options, __cronifyJob: true };
}
