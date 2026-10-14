import * as path from "node:path";
import * as vscode from "vscode";
import { Backend, BackendError } from "./backend";
import { SnapshotDocuments, snapshotUri } from "./documents";
import { HistoryView } from "./historyView";
import type { HistoryEntry, HistoryState } from "./protocol";

export function activate(context: vscode.ExtensionContext): { getState(): HistoryState } {
  const backend = new Backend(() => {
    const configured = vscode.workspace.getConfiguration("ghist").get<string>("backendPath", "").trim();
    if (configured && !path.isAbsolute(configured)) { throw new Error("ghist.backendPath must be an absolute executable path."); }
    return configured || context.asAbsolutePath(path.join("bin", process.platform === "win32" ? "ghist-backend.exe" : "ghist-backend"));
  });
  let navigation = 0;
  const history = new HistoryView(backend, context.extensionUri, open, () => { ++navigation; });
  context.subscriptions.push(backend, history,
    vscode.workspace.registerTextDocumentContentProvider("ghist", new SnapshotDocuments(backend)));

  async function open(candidate: HistoryEntry, file: boolean, preserveFocus: boolean): Promise<void> {
    const entry = history.resolve(candidate);
    if (!entry) { return; }
    const current = ++navigation;
    const v = entry.version;
    if (file && !v.afterPath) {
      throw new Error("This commit deleted the file. Open Changes to see its previous contents.");
    }
    const left = snapshotUri(entry, true);
    const right = snapshotUri(entry, false);
    // Resolve complete snapshots before opening a native editor. Cancellation
    // of history does not disturb already-open immutable documents.
    const documents = await Promise.all(file ? [vscode.workspace.openTextDocument(right)] :
      [vscode.workspace.openTextDocument(left), vscode.workspace.openTextDocument(right)]);
    if (current !== navigation) { return; }
    const column = await history.beside();
    if (current !== navigation) { return; }
    const options = { preview: true, preserveFocus, viewColumn: column };
    if (file) { await vscode.window.showTextDocument(documents[0], options); }
    else {
      await vscode.commands.executeCommand("vscode.diff", left, right,
        `${v.afterPath || v.beforePath} (${v.parent ? v.parent.slice(0, 7) : "empty"} ↔ ${v.shortHash})`, options);
    }
    // Opening a diff in a newly-created group can activate that group even
    // with preserveFocus. Restore the History panel after the editor command
    // has completed so keyboard navigation remains in the webview.
    if (preserveFocus && current === navigation) { history.focus(); }
  }

  const command = <Args extends unknown[]>(name: string, handler: (...args: Args) => unknown) => {
    context.subscriptions.push(vscode.commands.registerCommand(name, async (...args: Args) => {
      try {
        if (!vscode.workspace.isTrusted) { throw new Error("Trust this workspace to use ghist."); }
        return await handler(...args);
      } catch (error) {
        if (error instanceof BackendError && error.code === "cancelled") { return; }
        void vscode.window.showErrorMessage(`ghist: ${error instanceof Error ? error.message : String(error)}`);
      }
    }));
  };

  command("ghist.fileHistory", (resource?: vscode.Uri) => history.show(resource instanceof vscode.Uri ? resource : undefined));
  command("ghist.refresh", () => history.refresh());
  command("ghist.openDiff", async (candidate?: HistoryEntry) => {
    const entry = history.resolve(candidate);
    if (entry) { history.controller.select(entry.version.hash); await open(entry, false, true); }
  });
  command("ghist.openFile", async (candidate?: HistoryEntry) => {
    const entry = history.resolve(candidate);
    if (entry) { history.controller.select(entry.version.hash); await open(entry, true, false); }
  });
  // Exposed through the extension API for integration checks; it is not a
  // command that accepts data or bypasses message validation.
  return Object.freeze({ getState: () => structuredClone(history.controller.state) });
}
