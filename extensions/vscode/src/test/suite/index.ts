import * as assert from "node:assert/strict";
import { execFile, execFileSync } from "node:child_process";
import { writeFileSync } from "node:fs";
import * as path from "node:path";
import * as vscode from "vscode";
import { snapshotUri } from "../../documents";
import type { FileHistory, HistoryEntry, HistoryState } from "../../protocol";

// Runs inside an actual VS Code extension host, rather than a mocked API.
export async function run(): Promise<void> {
  const root = process.env.GHIST_TEST_REPOSITORY;
  assert.ok(root);
  const extension = vscode.extensions.getExtension("lcaohoanq.ghist");
  assert.ok(extension, "extension was not discovered");
  const api = await extension.activate() as { getState(): HistoryState };
  const waitFor = async (predicate: () => boolean) => {
    for (let i = 0; i < 100 && !predicate(); i++) { await new Promise(resolve => setTimeout(resolve, 100)); }
    assert.ok(predicate(), "timed out waiting for History state");
  };
  const file = vscode.Uri.file(path.join(root, "renamed [x] $file.txt"));
  const history = await vscode.commands.executeCommand<FileHistory>("ghist.fileHistory", file);
  assert.ok(history, "history command failed");
  assert.deepEqual(history.versions.map(v => v.subject), ["edit", "rename", "create"]);
  await waitFor(() => vscode.window.tabGroups.activeTabGroup.activeTab?.input instanceof vscode.TabInputWebview);
  assert.equal(vscode.window.tabGroups.all.length, 1);
  const historyGroup = vscode.window.tabGroups.activeTabGroup;
  assert.ok(historyGroup.activeTab?.input instanceof vscode.TabInputWebview);
  await new Promise<void>((resolve, reject) => {
    execFile(process.execPath, [path.resolve(__dirname, "../../../scripts/drive-history-smoke.mjs")], {
      env: { ...process.env, ELECTRON_RUN_AS_NODE: "1" }, timeout: 30000,
    }, (error, stdout, stderr) => { console.log(stdout); if (error) { reject(new Error(`${error.message}\n${stderr}`)); } else { resolve(); } });
  });
  await waitFor(() => api.getState().pinned && api.getState().selectedHash === history.versions[1].hash);
  if (process.env.GHIST_TEST_REVIEWS) {
    await waitFor(() => api.getState().reviews?.[history.versions[1].hash]?.[0]?.number === 42);
  }
  await waitFor(() => vscode.window.tabGroups.all.some(g => g.tabs.some(t => t.input instanceof vscode.TabInputTextDiff)));
  assert.equal(vscode.window.tabGroups.activeTabGroup, historyGroup, "click preview moved focus from History");
  assert.equal(vscode.window.tabGroups.all.length, 2, "preview did not create exactly one adjacent group");
  const otherFile = vscode.Uri.file(path.join(root, "other.txt"));
  writeFileSync(otherFile.fsPath, "other\n");
  execFileSync("git", ["add", "other.txt"], { cwd: root });
  execFileSync("git", ["commit", "-m", "other"], { cwd: root });
  await vscode.window.showTextDocument(otherFile, { viewColumn: historyGroup.viewColumn });
  await new Promise(resolve => setTimeout(resolve, 400));
  assert.equal(api.getState().target, file.fsPath, "pin failed to keep the original file");
  await vscode.commands.executeCommand("ghist.fileHistory");
  assert.equal(api.getState().target, file.fsPath, "reopening History changed the pinned target");
  await new Promise<void>((resolve, reject) => {
    execFile(process.execPath, [path.resolve(__dirname, "../../../scripts/drive-history-smoke.mjs"), "unpin"], {
      env: { ...process.env, ELECTRON_RUN_AS_NODE: "1" }, timeout: 30000,
    }, error => { if (error) { reject(error); } else { resolve(); } });
  });
  await waitFor(() => api.getState().history?.path === "other.txt");
  assert.equal(api.getState().selectedHash, undefined, "unpin/new file retained an old selection");
  const diffGroup = vscode.window.tabGroups.all.find(g => g !== historyGroup)!;
  await vscode.window.showTextDocument(file, { viewColumn: diffGroup.viewColumn });
  await waitFor(() => api.getState().history?.path === "renamed [x] $file.txt");
  assert.equal(api.getState().selectedHash, undefined, "automatic follow opened or selected a diff");
  await vscode.commands.executeCommand("ghist.fileHistory", file);
  for (const version of history.versions) {
    const entry: HistoryEntry = { root: history.root, version };
    await vscode.commands.executeCommand("ghist.openDiff", entry);
    const tab = vscode.window.tabGroups.all.find(g => g !== historyGroup)?.activeTab;
    assert.ok(tab?.input instanceof vscode.TabInputTextDiff, "native diff editor did not open");
    const left = await vscode.workspace.openTextDocument(snapshotUri(entry, true));
    const right = await vscode.workspace.openTextDocument(snapshotUri(entry, false));
    assert.equal(right.getText(), version.subject === "edit" ? "first\nchanged\nthird\n" : "first\nsecond\nthird\n");
    assert.equal(left.getText(), version.subject === "create" ? "" : "first\nsecond\nthird\n");
    // URI roundtrips must preserve spaces, brackets, dollar signs and JSON.
    assert.equal(vscode.Uri.parse(right.uri.toString()).query, right.uri.query);
    await vscode.commands.executeCommand("ghist.openFile", entry);
    assert.equal(vscode.window.activeTextEditor?.document.uri.scheme, "ghist");
    assert.equal(vscode.window.tabGroups.all.length, 2, "navigation added an editor group");
    assert.equal(api.getState().target, file.fsPath, "snapshot navigation changed the tracked working file");
  }
  // Rapid navigation must not let an older asynchronous diff steal focus.
  const newest = { root: history.root, version: history.versions[0] };
  const oldest = { root: history.root, version: history.versions[2] };
  await Promise.all([
    vscode.commands.executeCommand("ghist.openDiff", newest),
    vscode.commands.executeCommand("ghist.openFile", oldest),
  ]);
  assert.equal(vscode.window.activeTextEditor?.document.uri.toString(), snapshotUri(oldest, false).toString());
  const refreshed = await vscode.commands.executeCommand<FileHistory>("ghist.refresh");
  assert.equal(refreshed?.versions[0].hash, history.versions[0].hash);
  assert.equal(api.getState().selectedHash, oldest.version.hash, "refresh lost the selected commit");
  const git = (...args: string[]) => execFileSync("git", args, {
    cwd: root, encoding: "utf8",
    env: { ...process.env, GIT_CONFIG_NOSYSTEM: "1", GIT_CONFIG_GLOBAL: path.join(root, "absent-config") },
  }).trim();
  git("rm", "renamed [x] $file.txt");
  git("commit", "-m", "delete");
  const deleted = await vscode.commands.executeCommand<FileHistory>("ghist.refresh");
  assert.ok(deleted);
  assert.equal(deleted.versions[0].afterPath, "");
  assert.equal(deleted.versions.length, 4);
  const deletion: HistoryEntry = { root: deleted.root, version: deleted.versions[0] };
  await vscode.commands.executeCommand("ghist.openDiff", deletion);
  assert.equal((await vscode.workspace.openTextDocument(snapshotUri(deletion, true))).getText(), "first\nchanged\nthird\n");
  assert.equal((await vscode.workspace.openTextDocument(snapshotUri(deletion, false))).getText(), "");
  const stable = { root: history.root, version: history.versions[0] };
  assert.equal((await vscode.workspace.openTextDocument(snapshotUri(stable, false))).getText(), "first\nchanged\nthird\n");
  for (const [name, content, error] of [
    ["binary.txt", "\0\x01", /binary/],
    ["oversized.txt", "a".repeat((2 << 20) + 1), /2 MiB/],
  ] as const) {
    writeFileSync(path.join(root, name), content);
    git("add", name); git("commit", "-m", name);
    const entry: HistoryEntry = {
      root: history.root,
      version: { ...history.versions[0], hash: git("rev-parse", "HEAD"), afterPath: name },
    };
    await assert.rejects(Promise.resolve(vscode.workspace.openTextDocument(snapshotUri(entry, false))), error);
  }
  console.log("ghist extension host smoke passed: PR lookup/badge, webview click/keyboard/pin, adjacent diff focus/groups, history, rename, add/delete diffs, stable snapshots, refresh, binary and oversized rejection");
  await vscode.commands.executeCommand("workbench.action.closeAllEditors");
}
