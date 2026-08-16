import { randomUUID } from "node:crypto";
import { mkdir, readFile, unlink, writeFile } from "node:fs/promises";
import { join } from "node:path";

export interface LockStore {
  acquire(key: string, token: string, ttlSeconds: number): Promise<boolean>;
  release(key: string, token: string): Promise<void>;
}

export class LockHeldError extends Error {
  constructor(public readonly key: string) {
    super(`Lock "${key}" is already held`);
  }
}

export interface WithLockOptions {
  key: string;
  store: LockStore;
  ttlSeconds?: number;
  onLocked?: "skip" | "throw";
}

const DEFAULT_TTL_SECONDS = 300;

export function withLock(handler: () => void | Promise<void>, options: WithLockOptions): () => Promise<void> {
  const { key, store, ttlSeconds = DEFAULT_TTL_SECONDS, onLocked = "skip" } = options;

  return async function lockedHandler(): Promise<void> {
    const token = randomUUID();
    const acquired = await store.acquire(key, token, ttlSeconds);
    if (!acquired) {
      if (onLocked === "throw") throw new LockHeldError(key);
      return;
    }
    try {
      await handler();
    } finally {
      await store.release(key, token);
    }
  };
}

export function createMemoryLockStore(): LockStore {
  const locks = new Map<string, { token: string; expiresAt: number }>();

  return {
    async acquire(key, token, ttlSeconds) {
      const now = Date.now();
      const existing = locks.get(key);
      if (existing && existing.expiresAt > now) return false;
      locks.set(key, { token, expiresAt: now + ttlSeconds * 1000 });
      return true;
    },
    async release(key, token) {
      const existing = locks.get(key);
      if (existing && existing.token === token) locks.delete(key);
    },
  };
}

export interface FileLockStoreOptions {
  dir?: string;
}

function lockFilePath(dir: string, key: string): string {
  return join(dir, `${encodeURIComponent(key)}.lock`);
}

async function readLockFile(file: string): Promise<{ token: string; expiresAt: number } | null> {
  try {
    return JSON.parse(await readFile(file, "utf8"));
  } catch {
    return null;
  }
}

export function createFileLockStore(options: FileLockStoreOptions = {}): LockStore {
  const dir = options.dir ?? join(process.cwd(), ".cronify-locks");

  return {
    async acquire(key, token, ttlSeconds) {
      await mkdir(dir, { recursive: true });
      const file = lockFilePath(dir, key);
      const expiresAt = Date.now() + ttlSeconds * 1000;

      try {
        await writeFile(file, JSON.stringify({ token, expiresAt }), { flag: "wx" });
        return true;
      } catch (error) {
        if ((error as NodeJS.ErrnoException).code !== "EEXIST") throw error;
      }

      const existing = await readLockFile(file);
      if (existing && existing.expiresAt <= Date.now()) {
        await writeFile(file, JSON.stringify({ token, expiresAt }), "utf8");
        return true;
      }
      return false;
    },

    async release(key, token) {
      const file = lockFilePath(dir, key);
      const existing = await readLockFile(file);
      if (existing && existing.token === token) {
        await unlink(file).catch(() => {});
      }
    },
  };
}
