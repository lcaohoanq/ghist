import { randomBytes } from "node:crypto";
import * as vscode from "vscode";
import { BackendError, type Backend } from "./backend";
import { HistoryController } from "./historyState";
import { historyHtml } from "./historyHtml";
import type { FileHistory, HistoryEntry, Reviews } from "./protocol";

export class HistoryView implements vscode.Disposable {
  readonly controller: HistoryController;
  private panel?: vscode.WebviewPanel;
  private latestFile?: vscode.Uri;
  private diffGroup?: vscode.TabGroup;
  private reviewRequest?: AbortController;
  private reviewTimer?: ReturnType<typeof setTimeout>;
  private reviewContext?: number;
  private readonly subscriptions: vscode.Disposable[] = [];

  constructor(private readonly backend: Backend, private readonly extensionUri: vscode.Uri,
    private readonly open: (entry: HistoryEntry, file: boolean, preserveFocus: boolean) => Promise<void>,
    invalidateNavigation: () => void) {
    this.controller = new HistoryController(
      (file, signal) => backend.request<FileHistory>({ operation: "history", filePath: file }, signal),
      () => this.publish(), invalidateNavigation);
    this.subscriptions.push(vscode.window.onDidChangeActiveTextEditor(editor => this.track(editor)));
    this.track(vscode.window.activeTextEditor);
  }

  private track(editor: vscode.TextEditor | undefined): void {
    // Native diffs can surface a text editor event. Never treat either side as
    // a working file, including ordinary Git diffs opened elsewhere.
    if (!editor || editor.document.uri.scheme !== "file" ||
      vscode.window.tabGroups.activeTabGroup.activeTab?.input instanceof vscode.TabInputTextDiff) { return; }
    this.latestFile = editor.document.uri;
    this.controller.workingFile(editor.document.uri.fsPath);
  }

  async show(uri?: vscode.Uri): Promise<FileHistory | undefined> {
    uri ??= this.controller.state.pinned && this.controller.state.target
      ? vscode.Uri.file(this.controller.state.target) : this.latestFile;
    if (uri && uri.scheme !== "file") { throw new Error("Select a file in a local or remote Git working tree."); }
    if (!this.panel) {
      const media = vscode.Uri.joinPath(this.extensionUri, "media");
      const scripts = vscode.Uri.joinPath(this.extensionUri, "out", "client", "webview");
      const panel = vscode.window.createWebviewPanel("ghist.history", "ghist History", vscode.ViewColumn.Active, {
        enableScripts: true, localResourceRoots: [media, scripts],
      });
      this.panel = panel;
      panel.webview.html = historyHtml(
        panel.webview.asWebviewUri(vscode.Uri.joinPath(media, "history.css")).toString(),
        panel.webview.asWebviewUri(vscode.Uri.joinPath(scripts, "history.js")).toString(),
        panel.webview.cspSource, randomBytes(24).toString("base64"));
      const listeners = [
        panel.webview.onDidReceiveMessage((message: unknown) => { void this.message(message); }),
        panel.onDidChangeViewState(() => {
          this.controller.setVisible(panel.visible);
          if (!panel.visible) {
            this.cancelReviews();
            this.controller.state.reviewLoadingHash = undefined;
          }
          this.publish();
        }),
      ];
      panel.onDidDispose(() => {
        for (const listener of listeners) { listener.dispose(); }
        this.panel = undefined;
        this.controller.setVisible(false);
      });
    } else { this.panel.reveal(this.panel.viewColumn); }
    this.controller.setVisible(true);
    return uri ? this.controller.load(uri.fsPath, uri.fsPath === this.controller.state.target) : undefined;
  }

  async choose(): Promise<void> {
    const selected = await vscode.window.showOpenDialog({ canSelectMany: false, canSelectFolders: false, openLabel: "Show File History" });
    if (selected?.[0]) { await this.show(selected[0]); }
  }

  private async message(value: unknown): Promise<void> {
    if (!value || typeof value !== "object") { return; }
    const message = value as { action?: unknown; hash?: unknown; context?: unknown; index?: unknown };
    if (message.action === "ready") { this.publish(); return; }
    if (!vscode.workspace.isTrusted || message.context !== this.controller.state.context) { return; }
    switch (message.action) {
      case "choose": await this.choose(); break;
      case "pin": this.controller.togglePin(); break;
      case "refresh": await this.refresh(); break;
      case "select": {
        const entry = this.controller.select(message.hash);
        if (entry) { this.scheduleReviews(entry); }
        break;
      }
      case "openReview": await this.openReview(message.hash, message.index); break;
      case "diff":
      case "file": {
        const context = this.controller.state.context;
        const entry = this.controller.select(message.hash);
        if (entry) {
          this.scheduleReviews(entry);
          try { await this.open(entry, message.action === "file", message.action === "diff"); }
          catch (error) {
            // Keep snapshot errors actionable in the panel, including when a
            // user rapidly moves on before a binary revision is resolved.
            if (context === this.controller.state.context && this.controller.selected()?.version.hash === entry.version.hash) {
              this.controller.state.error = error instanceof Error ? error.message : String(error);
              this.publish();
            }
          }
        }
        break;
      }
    }
  }

  private scheduleReviews(entry: HistoryEntry): void {
    this.cancelReviews();
    this.controller.state.reviewLoadingHash = undefined;
    this.controller.state.reviewError = undefined;
    if (Object.hasOwn(this.controller.state.reviews ?? {}, entry.version.hash)) {
      this.publish();
      return;
    }
    const context = this.controller.state.context;
    this.reviewContext = context;
    this.reviewTimer = setTimeout(() => {
      this.reviewTimer = undefined;
      if (!this.controller.resolve(entry.version.hash, context)) { return; }
      const controller = new AbortController();
      this.reviewRequest = controller;
      this.controller.state.reviewLoadingHash = entry.version.hash;
      this.publish();
      void this.backend.request<Reviews>({ operation: "reviews", root: entry.root, revision: entry.version.hash }, controller.signal)
        .then(result => {
          if (this.reviewRequest !== controller || !this.controller.resolve(entry.version.hash, context)) { return; }
          this.controller.state.reviews ??= {};
          this.controller.state.reviews[entry.version.hash] = result.links;
        })
        .catch(error => {
          if (this.reviewRequest !== controller || error instanceof BackendError && error.code === "cancelled") { return; }
          if (this.controller.resolve(entry.version.hash, context)) {
            this.controller.state.reviewError = error instanceof Error ? error.message : String(error);
          }
        })
        .finally(() => {
          if (this.reviewRequest === controller) {
            this.reviewRequest = undefined;
            this.controller.state.reviewLoadingHash = undefined;
            this.publish();
          }
        });
    }, 300);
    this.publish();
  }

  private async openReview(hash: unknown, index: unknown): Promise<void> {
    const link = this.controller.review(hash, index);
    if (!link) { return; }
    const uri = vscode.Uri.parse(link.url, true);
    if ((uri.scheme !== "https" && uri.scheme !== "http") || !uri.authority) { return; }
    await vscode.env.openExternal(uri);
  }

  private publish(): void {
    if (this.reviewContext !== undefined && this.reviewContext !== this.controller.state.context) {
      this.cancelReviews();
    }
    if (this.panel?.visible) { void this.panel.webview.postMessage({ type: "state", state: this.controller.state }); }
  }

  private cancelReviews(): void {
    if (this.reviewTimer) { clearTimeout(this.reviewTimer); this.reviewTimer = undefined; }
    this.reviewRequest?.abort();
    this.reviewRequest = undefined;
    this.reviewContext = undefined;
  }

  refresh(): Promise<FileHistory | undefined> {
    this.cancelReviews();
    this.controller.state.reviewLoadingHash = undefined;
    this.controller.state.reviewError = undefined;
    return this.controller.refresh();
  }
  selected(): HistoryEntry | undefined { return this.controller.selected(); }
  focus(): void { this.panel?.reveal(this.panel.viewColumn); }
  resolve(entry: HistoryEntry | undefined): HistoryEntry | undefined {
    if (!entry) { return this.selected(); }
    if (entry.root !== this.controller.state.history?.root) { return undefined; }
    return this.controller.resolve(entry.version?.hash);
  }

  async beside(): Promise<vscode.ViewColumn> {
    const own = this.panel?.viewColumn ?? vscode.window.tabGroups.activeTabGroup.viewColumn;
    // Prefer the adjacent existing group; create one only when necessary. An
    // explicit column keeps repeated previews from adding groups.
    let group = vscode.window.tabGroups.all.find(g => g === this.diffGroup && g.viewColumn !== own)
      ?? vscode.window.tabGroups.all.find(g => g.viewColumn > own)
      ?? vscode.window.tabGroups.all.find(g => g.viewColumn !== own);
    if (!group) {
      this.panel?.reveal(own);
      await vscode.commands.executeCommand("workbench.action.newGroupRight");
      group = vscode.window.tabGroups.activeTabGroup;
      this.panel?.reveal(own);
    }
    this.diffGroup = group;
    return group.viewColumn;
  }

  dispose(): void {
    this.cancelReviews();
    this.controller.dispose();
    this.panel?.dispose();
    for (const subscription of this.subscriptions) { subscription.dispose(); }
  }
}
