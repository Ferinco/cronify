import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { LockHeldError, createFileLockStore, createMemoryLockStore, withLock, type LockStore } from "../src/lock.js";

let fileLockDir = "";

const stores: Array<{ name: string; create: () => LockStore }> = [
  { name: "memory", create: () => createMemoryLockStore() },
  {
    name: "file",
    create: () => {
      fileLockDir = mkdtempSync(join(tmpdir(), "cronify-lock-"));
      return createFileLockStore({ dir: fileLockDir });
    },
  },
];

for (const { name, create } of stores) {
  describe(`${name} store`, () => {
    let store: LockStore;

    beforeEach(() => {
      store = create();
    });

    afterEach(() => {
      if (fileLockDir) rmSync(fileLockDir, { recursive: true, force: true });
      fileLockDir = "";
    });

    it("acquires a free lock and releases it", async () => {
      expect(await store.acquire("k", "t1", 60)).toBe(true);
      await store.release("k", "t1");
      expect(await store.acquire("k", "t2", 60)).toBe(true);
    });

    it("refuses to acquire a lock already held", async () => {
      expect(await store.acquire("k", "t1", 60)).toBe(true);
      expect(await store.acquire("k", "t2", 60)).toBe(false);
    });

    it("reclaims an expired lock", async () => {
      expect(await store.acquire("k", "t1", 0.05)).toBe(true);
      await new Promise((resolve) => setTimeout(resolve, 100));
      expect(await store.acquire("k", "t2", 60)).toBe(true);
    });

    it("does not release a lock it doesn't own (fencing)", async () => {
      expect(await store.acquire("k", "t1", 0.05)).toBe(true);
      await new Promise((resolve) => setTimeout(resolve, 100));
      expect(await store.acquire("k", "t2", 60)).toBe(true);

      await store.release("k", "t1");

      expect(await store.acquire("k", "t3", 60)).toBe(false);
    });
  });
}

describe("withLock", () => {
  it("runs the handler when the lock is free", async () => {
    const store = createMemoryLockStore();
    const handler = vi.fn(async () => {});
    const locked = withLock(handler, { key: "job", store });

    await locked();

    expect(handler).toHaveBeenCalledOnce();
  });

  it("releases the lock after the handler completes", async () => {
    const store = createMemoryLockStore();
    const locked = withLock(async () => {}, { key: "job", store });

    await locked();
    await locked();

    expect(await store.acquire("job", "probe", 60)).toBe(true);
  });

  it("releases the lock even when the handler throws", async () => {
    const store = createMemoryLockStore();
    const locked = withLock(
      async () => {
        throw new Error("boom");
      },
      { key: "job", store },
    );

    await expect(locked()).rejects.toThrow("boom");
    expect(await store.acquire("job", "probe", 60)).toBe(true);
  });

  it("skips the handler by default when the lock is held", async () => {
    const store = createMemoryLockStore();
    await store.acquire("job", "someone-else", 60);
    const handler = vi.fn(async () => {});
    const locked = withLock(handler, { key: "job", store });

    await locked();

    expect(handler).not.toHaveBeenCalled();
  });

  it("throws LockHeldError when onLocked is 'throw'", async () => {
    const store = createMemoryLockStore();
    await store.acquire("job", "someone-else", 60);
    const handler = vi.fn(async () => {});
    const locked = withLock(handler, { key: "job", store, onLocked: "throw" });

    await expect(locked()).rejects.toThrow(LockHeldError);
    expect(handler).not.toHaveBeenCalled();
  });

  it("calls onSkip when the lock is held, in both onLocked modes", async () => {
    const store = createMemoryLockStore();
    await store.acquire("job", "someone-else", 60);
    const onSkip = vi.fn();

    await withLock(async () => {}, { key: "job", store, onSkip })();
    expect(onSkip).toHaveBeenCalledOnce();

    await expect(withLock(async () => {}, { key: "job", store, onLocked: "throw", onSkip })()).rejects.toThrow(
      LockHeldError,
    );
    expect(onSkip).toHaveBeenCalledTimes(2);
  });

  it("does not call onSkip when the lock is free", async () => {
    const store = createMemoryLockStore();
    const onSkip = vi.fn();
    const locked = withLock(async () => {}, { key: "job", store, onSkip });

    await locked();

    expect(onSkip).not.toHaveBeenCalled();
  });
});
