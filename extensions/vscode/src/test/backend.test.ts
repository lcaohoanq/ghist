import * as assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import * as path from "node:path";
import { test } from "node:test";
import { Backend, BackendError } from "../backend";
import type { FileHistory, Snapshot } from "../protocol";

const executable = path.resolve(__dirname, "../../bin", process.platform === "win32" ? "ghist-backend.exe" : "ghist-backend");

test("real backend follows renames, returns exact parent contents, and never changes the worktree", async () => {
  const root = mkdtempSync(path.join(tmpdir(), "ghist backend space-"));
  const backend = new Backend(() => executable);
  const git = (...args: string[]) => execFileSync("git", args, {
    cwd: root, encoding: "utf8",
    env: { ...process.env, GIT_CONFIG_NOSYSTEM: "1", GIT_CONFIG_GLOBAL: path.join(root, "absent-config") },
  }).trim();
  try {
    git("init", "-b", "main");
    git("config", "user.name", "Test");
    git("config", "user.email", "test@example.com");
    git("config", "commit.gpgsign", "false");
    const old = "old file.txt", renamed = "đổi [x] $file.txt";
    writeFileSync(path.join(root, old), "first\nsecond\nthird\n");
    git("add", "-A"); git("commit", "-m", "create");
    git("mv", old, renamed); git("commit", "-m", "rename");
    writeFileSync(path.join(root, renamed), "first\nchanged\nthird\n");
    git("add", "-A"); git("commit", "-m", "edit");
    writeFileSync(path.join(root, renamed), "uncommitted contents\n");
    const before = git("status", "--porcelain=v1");
    const history = await backend.request<FileHistory>({ operation: "history", filePath: path.join(root, renamed) });
    assert.deepEqual(history.versions.map(v => v.subject), ["edit", "rename", "create"]);
    assert.equal(history.versions[1].beforePath, old);
    assert.equal(history.versions[1].afterPath, renamed);
    const version = history.versions[0];
    const snapshot = await backend.request<Snapshot>({ operation: "snapshot", root: history.root, revision: version.parent, path: version.beforePath });
    assert.equal(snapshot.content, "first\nsecond\nthird\n");
    const after = await backend.request<Snapshot>({ operation: "snapshot", root: history.root, revision: version.hash, path: version.afterPath });
    assert.equal(after.content, "first\nchanged\nthird\n");
    assert.equal(git("status", "--porcelain=v1"), before);
    await assert.rejects(backend.request({ operation: "snapshot", root, revision: "--help", path: renamed }), /invalid commit hash/);
    await assert.rejects(backend.request({ operation: "history", filePath: path.join(root, "missing") }),
      (error: unknown) => error instanceof BackendError && error.code === "no_history");
  } finally {
    backend.dispose();
    rmSync(root, { recursive: true, force: true });
  }
});

test("missing executables and cancellation reject without hanging", async () => {
  const missing = new Backend(() => path.join(tmpdir(), "ghist-no-such-backend", "missing"));
  await assert.rejects(missing.request({ operation: "history", filePath: __filename }),
    (error: unknown) => error instanceof BackendError && error.code === "start_failed");
  missing.dispose();
  const backend = new Backend(() => executable);
  const controller = new AbortController();
  const pending = backend.request({ operation: "history", filePath: __filename }, controller.signal);
  controller.abort();
  await assert.rejects(pending, (error: unknown) => error instanceof BackendError && error.code === "cancelled");
  await assert.rejects(backend.request({ operation: "history", filePath: __filename }, controller.signal), /cancelled/);
  backend.dispose();
  await assert.rejects(backend.request({ operation: "history", filePath: __filename }), /cancelled/);
});
