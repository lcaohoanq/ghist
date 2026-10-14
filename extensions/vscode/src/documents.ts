import * as vscode from "vscode";
import type { Backend } from "./backend";
import type { HistoryEntry, Snapshot } from "./protocol";

interface SnapshotAddress {
  root: string;
  revision: string;
  path: string;
  empty: boolean;
}

export function snapshotUri(entry: HistoryEntry, before: boolean): vscode.Uri {
  const v = entry.version;
  const path = before ? v.beforePath : v.afterPath;
  const revision = before ? v.parent : v.hash;
  return vscode.Uri.from({
    scheme: "ghist",
    path: `/${(revision || v.hash).slice(0, 7)}/${path || v.afterPath || v.beforePath}`,
    query: JSON.stringify({ root: entry.root, revision, path, empty: !path || !revision } satisfies SnapshotAddress),
  });
}

export class SnapshotDocuments implements vscode.TextDocumentContentProvider {
  constructor(private readonly backend: Backend) {}

  async provideTextDocumentContent(uri: vscode.Uri, token: vscode.CancellationToken): Promise<string> {
    if (!vscode.workspace.isTrusted) { throw new Error("Trust this workspace to use ghist."); }
    const address = JSON.parse(uri.query) as SnapshotAddress;
    if (typeof address.root !== "string" || typeof address.revision !== "string" || typeof address.path !== "string" || typeof address.empty !== "boolean") {
      throw new Error("Invalid ghist document address.");
    }
    if (address.empty) { return ""; }
    const controller = new AbortController();
    const listener = token.onCancellationRequested(() => controller.abort());
    if (token.isCancellationRequested) { controller.abort(); }
    try {
      const snapshot = await this.backend.request<Snapshot>({
        operation: "snapshot", root: address.root, revision: address.revision, path: address.path,
      }, controller.signal);
      if (snapshot.binary) { throw new Error("This revision is binary or is not UTF-8 text; ghist cannot display it as text."); }
      // Refuse partial text: it would make the native diff report false changes.
      if (snapshot.truncated) { throw new Error("This revision exceeds the 2 MiB text limit. ghist did not open an incomplete diff."); }
      return snapshot.content;
    } finally {
      listener.dispose();
    }
  }
}
