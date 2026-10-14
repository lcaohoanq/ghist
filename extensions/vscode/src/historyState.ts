import { BackendError } from "./backend";
import type { FileHistory, HistoryEntry, HistoryState, ReviewLink } from "./protocol";

// The host owns all data and selection. A context identifies the displayed
// dataset, so messages from an old webview render cannot select another file.
export class HistoryController {
  readonly state: HistoryState = { context: 0, pinned: false, loading: false };
  private latestFile?: string;
  private visible = false;
  private pendingFollow?: ReturnType<typeof setTimeout>;
  private request?: AbortController;
  private needsReload = false;
  private disposed = false;

  constructor(
    private readonly fetch: (file: string, signal: AbortSignal) => Promise<FileHistory>,
    private readonly changed: () => void,
    private readonly invalidateNavigation: () => void,
  ) {}

  workingFile(file: string): void {
    this.latestFile = file;
    this.scheduleFollow();
  }

  setVisible(visible: boolean): void {
    if (this.visible === visible) { return; }
    this.visible = visible;
    if (!visible) {
      this.invalidateNavigation();
      this.clearFollow();
      if (this.request) { this.cancel(); this.needsReload = true; this.changed(); }
      return;
    }
    // Resume once, with the most recent working file rather than a hidden tab's
    // stale request. No automatic fetch occurs before the panel is opened.
    if (!this.state.pinned && this.latestFile && this.latestFile !== this.state.target) {
      this.scheduleFollow();
    } else if (this.needsReload && this.state.target) {
      void this.load(this.state.target, true);
    }
  }

  togglePin(): void {
    this.state.pinned = !this.state.pinned;
    this.clearFollow();
    this.changed();
    if (!this.state.pinned) { this.scheduleFollow(); }
  }

  private scheduleFollow(): void {
    this.clearFollow();
    if (this.disposed || this.state.pinned || !this.visible || !this.latestFile) { return; }
    if (this.latestFile === this.state.target && !this.needsReload) { return; }
    if (this.request) { this.cancel(); this.needsReload = true; this.changed(); }
    // Invalidate immediately; a pending snapshot must not open after the user
    // has moved to another working file during the debounce window.
    this.invalidateNavigation();
    this.pendingFollow = setTimeout(() => {
      this.pendingFollow = undefined;
      if (this.latestFile) { void this.load(this.latestFile, this.latestFile === this.state.target); }
    }, 250);
  }

  private clearFollow(): void {
    if (this.pendingFollow) { clearTimeout(this.pendingFollow); this.pendingFollow = undefined; }
  }

  private cancel(): void {
    this.request?.abort();
    this.request = undefined;
    this.state.loading = false;
  }

  async load(file: string, preserve = false): Promise<FileHistory | undefined> {
    if (this.disposed) { return undefined; }
    this.clearFollow();
    this.cancel();
    this.invalidateNavigation();
    const sameFile = this.state.target === file;
    if (!preserve || !sameFile) {
      this.state.history = undefined;
      this.state.selectedHash = undefined;
      this.state.reviews = {};
      this.state.reviewLoadingHash = undefined;
      this.state.reviewError = undefined;
      ++this.state.context;
    }
    this.state.target = file;
    this.state.error = undefined;
    this.state.empty = false;
    this.state.loading = true;
    this.needsReload = false;
    const controller = new AbortController();
    this.request = controller;
    this.changed();
    try {
      const history = await this.fetch(file, controller.signal);
      if (this.request !== controller || this.disposed) { return undefined; }
      this.state.history = history;
      this.state.empty = history.versions.length === 0;
      const hashes = new Set(history.versions.map(v => v.hash));
      this.state.reviews = Object.fromEntries(Object.entries(this.state.reviews ?? {}).filter(([hash]) => hashes.has(hash)));
      if (!history.versions.some(v => v.hash === this.state.selectedHash)) { this.state.selectedHash = undefined; }
      ++this.state.context;
      return history;
    } catch (error) {
      if (this.request !== controller || this.disposed) { return undefined; }
      if (error instanceof BackendError && error.code === "no_history") {
        this.state.empty = true;
        this.state.history = undefined;
        this.state.selectedHash = undefined;
        ++this.state.context;
      } else {
        this.state.error = error instanceof Error ? error.message : String(error);
      }
      return undefined;
    } finally {
      if (this.request === controller) {
        this.request = undefined;
        this.state.loading = false;
        this.changed();
      }
    }
  }

  refresh(): Promise<FileHistory | undefined> {
    return this.state.target ? this.load(this.state.target, true) : Promise.resolve(undefined);
  }

  resolve(hash: unknown, context = this.state.context): HistoryEntry | undefined {
    if (typeof hash !== "string" || context !== this.state.context) { return undefined; }
    const version = this.state.history?.versions.find(v => v.hash === hash);
    return version && this.state.history ? { root: this.state.history.root, version } : undefined;
  }

  select(hash: unknown, context = this.state.context): HistoryEntry | undefined {
    const entry = this.resolve(hash, context);
    if (entry) {
      this.invalidateNavigation();
      this.state.selectedHash = entry.version.hash;
      this.changed();
    }
    return entry;
  }

  selected(): HistoryEntry | undefined { return this.resolve(this.state.selectedHash); }

  review(hash: unknown, index: unknown): ReviewLink | undefined {
    const entry = this.resolve(hash);
    if (!entry || !Number.isInteger(index) || (index as number) < 0) { return undefined; }
    return this.state.reviews?.[entry.version.hash]?.[index as number];
  }

  dispose(): void {
    this.disposed = true;
    this.clearFollow();
    this.cancel();
    this.invalidateNavigation();
  }
}
