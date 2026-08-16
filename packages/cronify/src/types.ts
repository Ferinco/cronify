export interface DefineJobOptions {
  /** Unique job id. Becomes the route segment /api/cron/<id>. Letters, numbers, hyphens only. */
  id: string;
  /** Standard 5-field cron expression (minute hour day month weekday). */
  schedule: string;
  handler: () => void | Promise<void>;
  /** Per-attempt timeout enforced by the scheduler, in seconds. Default 30. */
  timeoutSeconds?: number;
  /** Total attempts (1 initial + retries) the scheduler will make. Default 3. */
  maxAttempts?: number;
  /** When false, the route rejects requests and the scheduler won't fire it. Default true. */
  enabled?: boolean;
  description?: string;
}

export interface JobDefinition extends DefineJobOptions {
  readonly __cronifyJob: true;
}

export interface ManifestJobEntry {
  id: string;
  schedule: string;
  route: string;
  enabled: boolean;
  timeoutSeconds: number;
  maxAttempts: number;
  description: string | null;
}

export interface Manifest {
  version: 1;
  generatedAt: string;
  jobs: ManifestJobEntry[];
}

export const DEFAULT_TIMEOUT_SECONDS = 30;
export const DEFAULT_MAX_ATTEMPTS = 3;
export const MANIFEST_FILENAME = ".cron-manifest.json";
