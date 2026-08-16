export interface DefineJobOptions {
  id: string;
  schedule: string;
  handler: () => void | Promise<void>;
  timeoutSeconds?: number;
  maxAttempts?: number;
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
