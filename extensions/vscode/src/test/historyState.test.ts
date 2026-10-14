import * as assert from "node:assert/strict";
import { test } from "node:test";
import { BackendError } from "../backend";
import { HistoryController } from "../historyState";
import type { FileHistory, FileVersion } from "../protocol";

function history(path: string, hashes = ["new", "old"]): FileHistory {
  return { root: "/repo", path, head: "head", versions: hashes.map(hash => ({
    hash, shortHash: hash, subject: hash, author: "Test", email: "", date: "2026-01-01", parent: "parent",
    beforePath: path, afterPath: path, change: "M", merge: false,
  } satisfies FileVersion)) };
}
function harness() {
  const requests: { file: string; signal: AbortSignal; resolve: (history: FileHistory) => void; reject: (error: Error) => void }[] = [];
  let changes = 0, navigations = 0;
  const controller = new HistoryController((file, signal) => new Promise((resolve, reject) => {
    requests.push({ file, signal, resolve, reject });
  }), () => { ++changes; }, () => { ++navigations; });
  return { controller, requests, counts: () => ({ changes, navigations }) };
}
const flush = async () => { await Promise.resolve(); await Promise.resolve(); };

test("follow is lazy, debounced, deferred while hidden, and pin/unpin follows the latest file", async t => {
  t.mock.timers.enable({ apis: ["setTimeout"] });
  const { controller: c, requests } = harness();
  t.after(() => c.dispose());
  c.workingFile("/repo/a"); t.mock.timers.tick(1000); assert.equal(requests.length, 0);
  c.setVisible(true); t.mock.timers.tick(249); assert.equal(requests.length, 0);
  c.workingFile("/repo/b"); t.mock.timers.tick(250);
  assert.equal(requests.length, 1); assert.equal(requests[0].file, "/repo/b");
  requests[0].resolve(history("b")); await flush();
  c.togglePin(); c.workingFile("/repo/c"); t.mock.timers.tick(1000); assert.equal(requests.length, 1);
  c.togglePin(); t.mock.timers.tick(250); assert.equal(requests[1].file, "/repo/c");
  c.setVisible(false); assert.equal(requests[1].signal.aborted, true);
  c.workingFile("/repo/d"); t.mock.timers.tick(1000); assert.equal(requests.length, 2);
  // A cancelled backend may still resolve. Its contents must not be accepted.
  requests[1].resolve(history("c")); await flush(); assert.notEqual(c.state.history?.path, "c");
  c.setVisible(true); t.mock.timers.tick(250); assert.equal(requests[2].file, "/repo/d");
  requests[2].resolve(history("d")); await flush(); assert.equal(c.state.history?.path, "d");
});

test("late results and stale webview identifiers are ignored; new files reset selection", async t => {
  const { controller: c, requests } = harness(); t.after(() => c.dispose());
  const first = c.load("/repo/a"); const second = c.load("/repo/b");
  assert.equal(requests[0].signal.aborted, true);
  requests[1].resolve(history("b")); await second;
  const context = c.state.context;
  requests[0].resolve(history("a")); await first; assert.equal(c.state.history?.path, "b");
  assert.equal(c.select("forged"), undefined); assert.equal(c.selected(), undefined);
  c.select("old"); assert.equal(c.selected()?.version.hash, "old");
  const third = c.load("/repo/c"); assert.equal(c.selected(), undefined);
  requests[2].resolve(history("c")); await third;
  assert.equal(c.select("old", context), undefined); assert.equal(c.selected(), undefined);
});

test("refresh retains contents and selection, clears removed commits, and exposes retryable errors", async t => {
  const { controller: c, requests } = harness(); t.after(() => c.dispose());
  let pending = c.load("/repo/a"); requests[0].resolve(history("a")); await pending;
  c.select("old");
  pending = c.refresh(); assert.equal(c.state.loading, true); assert.equal(c.selected()?.version.hash, "old");
  requests[1].resolve(history("a", ["newer", "new", "old"])); await pending;
  assert.equal(c.selected()?.version.hash, "old");
  pending = c.refresh(); requests[2].reject(new Error("Git failed")); await pending;
  assert.equal(c.state.error, "Git failed"); assert.equal(c.state.history?.versions.length, 3);
  assert.equal(c.selected()?.version.hash, "old"); assert.equal(c.state.loading, false);
  pending = c.refresh(); requests[3].resolve(history("a", ["new"])); await pending;
  assert.equal(c.selected(), undefined); assert.equal(c.state.error, undefined);
  pending = c.load("/repo/untracked"); requests[4].reject(new BackendError("no_history", "No history")); await pending;
  assert.equal(c.state.empty, true); assert.equal(c.state.history, undefined); assert.equal(c.state.error, undefined);
});

test("hiding and disposal cancel requests and prevent automatic work and late updates", async t => {
  t.mock.timers.enable({ apis: ["setTimeout"] });
  const { controller: c, requests, counts } = harness();
  c.setVisible(true); const pending = c.load("/repo/a");
  c.setVisible(false); assert.equal(requests[0].signal.aborted, true);
  requests[0].resolve(history("a")); await pending; assert.equal(c.state.history, undefined);
  c.setVisible(true); assert.equal(requests.length, 2);
  c.workingFile("/repo/b"); c.dispose(); const before = counts();
  requests[1].resolve(history("a")); await flush(); t.mock.timers.tick(1000);
  assert.equal(requests.length, 2); assert.deepEqual(counts(), before);
});

test("working file changes cancel in-flight history immediately and a rapid return resumes it", async t => {
  t.mock.timers.enable({ apis: ["setTimeout"] });
  const { controller: c, requests } = harness(); t.after(() => c.dispose());
  c.setVisible(true); const pending = c.load("/repo/a");
  c.workingFile("/repo/b"); assert.equal(requests[0].signal.aborted, true);
  requests[0].resolve(history("a")); await pending; assert.equal(Boolean(c.state.history), false);
  t.mock.timers.tick(100); c.workingFile("/repo/a");
  t.mock.timers.tick(249); assert.equal(requests.length, 1);
  t.mock.timers.tick(1); assert.equal(requests[1].file, "/repo/a");
  requests[1].resolve(history("a")); await flush(); assert.equal(c.state.history?.path, "a");
});

test("review links are resolved only for current commits and valid indexes", async t => {
  const { controller: c, requests } = harness(); t.after(() => c.dispose());
  const pending = c.load("/repo/a"); requests[0].resolve(history("a")); await pending;
  c.state.reviews = { old: [{ provider: "github", number: 12, url: "https://github.com/acme/repo/pull/12" }] };
  assert.equal(c.review("old", 0)?.number, 12);
  assert.equal(c.review("old", -1), undefined);
  assert.equal(c.review("old", 0.5), undefined);
  assert.equal(c.review("forged", 0), undefined);
  assert.equal(c.review("old", 1), undefined);
});
