import type { HistoryState, FileVersion } from "../protocol";

declare function acquireVsCodeApi(): {
  postMessage(message: unknown): void;
  getState(): { query?: string; scroll?: number; target?: string } | undefined;
  setState(state: unknown): void;
};
const api = acquireVsCodeApi();
const saved = api.getState();
const element = <T extends HTMLElement = HTMLElement>(id: string) => document.getElementById(id) as T;
const table = element("table");
const viewport = element("viewport");
const rows = element("rows");
const search = element<HTMLInputElement>("search");
const height = 36;
let state: HistoryState = { context: 0, pinned: false, loading: false };
let filtered: FileVersion[] = [];
let searchable: string[] = [];
let lastVersions: FileVersion[] | undefined;
let selected: string | undefined;
let frame = 0;
let emptyAction = "choose";
search.value = saved?.query || "";

function send(action: string, hash?: string, index?: number): void { api.postMessage({ action, hash, index, context: state.context }); }
function persist(): void { api.setState({ query: search.value, scroll: viewport.scrollTop, target: state.target }); }
function text(id: string, value: string): void { element(id).textContent = value; }
function enabled(id: string, value: boolean): void { element<HTMLButtonElement>(id).disabled = !value; }
function filter(): void {
  const query = search.value.trim().toLocaleLowerCase();
  filtered = (state.history?.versions || []).filter((_v, i) => !query || searchable[i].includes(query));
}
function controls(): void {
  const index = filtered.findIndex(v => v.hash === selected);
  enabled("newer", filtered.length > 0 && index !== 0);
  enabled("older", filtered.length > 0 && index !== filtered.length - 1);
  enabled("open-file", Boolean(state.history?.versions.find(v => v.hash === selected)?.afterPath));
  text("results", `${filtered.length.toLocaleString()} shown${selected ? " · " + selected.slice(0, 7) + " selected" : ""}`);
  const reviewStatus = element("review-status");
  const lookingUp = state.reviewLoadingHash === selected;
  reviewStatus.textContent = lookingUp ? "Looking up PR/MR…" : state.reviewError && selected ? "PR/MR unavailable" : "";
  reviewStatus.title = lookingUp ? "" : state.reviewError || "";
}
function fullTitle(v: FileVersion): string {
  return `${v.subject || "(no subject)"}\n${v.author} <${v.email}>\n${new Date(v.date).toLocaleString()}\n${v.hash}\n${v.beforePath && v.afterPath && v.beforePath !== v.afterPath ? v.beforePath + " → " + v.afterPath : v.afterPath || v.beforePath}${v.merge ? "\nMerge: diff compares against the first parent." : ""}`;
}
function cell(className: string, value = ""): HTMLDivElement {
  const node = document.createElement("div");
  node.className = className;
  node.setAttribute("role", "gridcell");
  node.textContent = value;
  return node;
}
function badge(label: string, type: string, title: string): HTMLSpanElement {
  const node = document.createElement("span");
  node.className = `badge ${type}`;
  node.textContent = label;
  node.title = title;
  return node;
}
function timeline(index: number, merge: boolean): HTMLDivElement {
  const node = cell("timeline");
  const ns = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(ns, "svg");
  svg.setAttribute("viewBox", "0 0 60 36");
  svg.setAttribute("aria-hidden", "true");
  const line = document.createElementNS(ns, "path");
  line.setAttribute("d", `M30 ${index === 0 ? 18 : 0}V${index === filtered.length - 1 ? 18 : 36}`);
  svg.append(line);
  // This timeline orders file versions. The history payload does not contain
  // the second-parent graph, so a branch here would end without a real node.
  // Mark merges with concentric rings on the same continuous timeline instead.
  const circle = document.createElementNS(ns, "circle");
  circle.setAttribute("cx", "30"); circle.setAttribute("cy", "18"); circle.setAttribute("r", merge ? "6" : "4");
  svg.append(circle);
  if (merge) {
    const inner = document.createElementNS(ns, "circle");
    inner.setAttribute("cx", "30"); inner.setAttribute("cy", "18"); inner.setAttribute("r", "2.5");
    svg.append(inner);
  }
  node.append(svg);
  return node;
}
function renderRows(): void {
  frame = 0;
  const count = filtered.length;
  rows.style.height = `${count * height}px`;
  const start = Math.max(0, Math.floor(viewport.scrollTop / height) - 8);
  const end = Math.min(count, Math.ceil((viewport.scrollTop + viewport.clientHeight) / height) + 8);
  const fragment = document.createDocumentFragment();
  for (let i = start; i < end; i++) {
    const v = filtered[i];
    const row = document.createElement("div");
    row.id = `commit-${i}`;
    row.className = `commit${v.hash === selected ? " selected" : ""}`;
    row.dataset.hash = v.hash;
    row.style.top = `${i * height}px`;
    row.setAttribute("role", "row");
    row.setAttribute("aria-rowindex", String(i + 2));
    row.setAttribute("aria-selected", String(v.hash === selected));
    row.title = fullTitle(v);
    row.append(timeline(i, v.merge));
    const message = cell("message");
    const subject = document.createElement("span");
    subject.className = "subject"; subject.textContent = v.subject || "(no subject)";
    message.append(subject);
    if (v.change === "A") { message.append(badge("Added", "added", "File added")); }
    if (v.change === "D") { message.append(badge("Deleted", "deleted", "File deleted")); }
    if (v.change.startsWith("R")) { message.append(badge("Renamed", "renamed", `${v.beforePath} → ${v.afterPath}`)); }
    if (v.merge) { message.append(badge("Merge", "merge", "Diff compares against the first parent")); }
    if (state.reviewLoadingHash === v.hash) { message.append(badge("…", "review-loading", "Looking up pull or merge requests")); }
    for (const [reviewIndex, review] of (state.reviews?.[v.hash] ?? []).entries()) {
      const link = document.createElement("button");
      link.className = "review-link";
      link.type = "button";
      link.dataset.reviewIndex = String(reviewIndex);
      link.textContent = review.provider === "gitlab" ? `MR !${review.number}` : `PR #${review.number}`;
      link.title = `Open ${link.textContent}: ${review.url}`;
      message.append(link);
    }
    const author = cell("author");
    const avatar = document.createElement("span");
    avatar.className = "avatar";
    avatar.textContent = v.author.trim().split(/\s+/).slice(0, 2).map(word => Array.from(word)[0] || "").join("").toLocaleUpperCase() || "?";
    avatar.setAttribute("aria-hidden", "true");
    const name = document.createElement("span"); name.textContent = v.author;
    author.append(avatar, name); author.title = `${v.author} <${v.email}>`;
    const time = cell("time", new Date(v.date).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" }));
    time.title = new Date(v.date).toLocaleString();
    const hash = cell("hash", v.shortHash); hash.title = v.hash;
    row.append(message, author, time, hash); fragment.append(row);
  }
  rows.replaceChildren(fragment);
  const activeIndex = filtered.findIndex(v => v.hash === selected);
  if (activeIndex >= start && activeIndex < end) { table.setAttribute("aria-activedescendant", `commit-${activeIndex}`); }
  else { table.removeAttribute("aria-activedescendant"); }
  table.setAttribute("aria-rowcount", String(count + 1));
  controls();
}
function scheduleRender(): void { if (!frame) { frame = requestAnimationFrame(renderRows); } }
function ensureVisible(): void {
  const index = filtered.findIndex(v => v.hash === selected);
  if (index < 0) { return; }
  if (index * height < viewport.scrollTop) { viewport.scrollTop = index * height; }
  else if ((index + 1) * height > viewport.scrollTop + viewport.clientHeight) { viewport.scrollTop = (index + 1) * height - viewport.clientHeight; }
}
function select(version: FileVersion | undefined, preview: boolean): void {
  if (!version) { return; }
  // Immediate visual feedback; the host confirms the canonical selection.
  selected = version.hash; ensureVisible(); renderRows();
  send(preview ? "diff" : "select", selected);
}
function move(delta: number): void {
  const index = filtered.findIndex(v => v.hash === selected);
  select(filtered[index < 0 ? (delta > 0 ? 0 : filtered.length - 1) : Math.max(0, Math.min(filtered.length - 1, index + delta))], false);
}
function renderState(next: HistoryState): void {
  const changedFile = state.target !== next.target;
  state = next;
  selected = state.selectedHash;
  if (changedFile && saved?.target !== state.target) { search.value = ""; viewport.scrollTop = 0; }
  text("repository", state.history?.root.split(/[\\/]/).pop() || "");
  text("file", state.history?.path || state.target || "Choose a file to explore its history");
  element("file").title = state.target || "";
  text("summary", state.history ? `${state.history.versions.length.toLocaleString()} commits · HEAD ${state.history.head.slice(0, 7)}` : "");
  element("loading").hidden = !state.loading;
  table.setAttribute("aria-busy", String(state.loading));
  text("pin", state.pinned ? "Unpin file" : "Pin file");
  element("pin").setAttribute("aria-pressed", String(state.pinned));
  element("pin").title = state.pinned ? "Unpin and follow the most recent working file" : "Pin this file; stop following the active editor";
  enabled("pin", Boolean(state.target)); enabled("refresh", Boolean(state.target));
  element("notice").hidden = !state.error;
  text("notice-text", state.error || "");
  if (lastVersions !== state.history?.versions) {
    lastVersions = state.history?.versions;
    searchable = (lastVersions || []).map(v => `${v.subject}\n${v.author}\n${v.email}\n${v.hash}`.toLocaleLowerCase());
  }
  filter();
  const noRows = filtered.length === 0;
  element("empty").hidden = !noRows;
  table.hidden = noRows;
  if (noRows) {
    emptyAction = state.error || state.empty ? "refresh" : "choose";
    if (state.error) {
      text("empty-title", "History could not be loaded"); text("empty-detail", "Retry, or choose another working file.");
    } else if (state.loading) {
      text("empty-title", "Loading history…"); text("empty-detail", "Reading committed file versions from Git.");
    } else if (!state.target) {
      text("empty-title", "Explore a file’s history"); text("empty-detail", "Choose a file, or open a working file in the editor. History follows it until you pin a file.");
    } else if (state.empty) {
      text("empty-title", "No committed history"); text("empty-detail", "This file has no commits at HEAD. Choose another file or refresh after committing.");
    } else {
      text("empty-title", "No matching commits"); text("empty-detail", "Try a different message, author or hash."); emptyAction = "clear";
    }
    text("empty-action", emptyAction === "clear" ? "Clear search" : emptyAction === "refresh" ? "Refresh history" : "Choose file");
  }
  renderRows();
  if (changedFile && saved && saved.target === state.target && saved.scroll) { viewport.scrollTop = saved.scroll; scheduleRender(); }
  persist();
}
for (const action of ["choose", "pin", "refresh"] as const) { element(action).addEventListener("click", () => send(action)); }
element("retry").addEventListener("click", () => send("refresh"));
element("empty-action").addEventListener("click", () => {
  if (emptyAction === "clear") { search.value = ""; renderState(state); search.focus(); }
  else { send(emptyAction); }
});
element("newer").addEventListener("click", () => move(-1));
element("older").addEventListener("click", () => move(1));
element("open-file").addEventListener("click", () => send("file", selected));
rows.addEventListener("click", event => {
  const target = event.target as Element;
  const row = target.closest<HTMLElement>(".commit");
  const hash = row?.dataset.hash;
  const review = target.closest<HTMLButtonElement>(".review-link");
  if (review && hash) {
    event.stopPropagation();
    send("openReview", hash, Number(review.dataset.reviewIndex));
    return;
  }
  select(filtered.find(v => v.hash === hash), true); table.focus({ preventScroll: true });
});
table.addEventListener("keydown", event => {
  if (event.key === "ArrowDown" || event.key === "ArrowUp") { event.preventDefault(); move(event.key === "ArrowDown" ? 1 : -1); }
  else if (event.key === "Enter" && selected) { event.preventDefault(); send("diff", selected); }
});
document.addEventListener("keydown", event => {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "f") { event.preventDefault(); search.focus(); search.select(); }
});
search.addEventListener("input", () => { viewport.scrollTop = 0; renderState(state); });
viewport.addEventListener("scroll", () => { scheduleRender(); persist(); });
new ResizeObserver(scheduleRender).observe(viewport);
window.addEventListener("message", event => {
  if (event.data?.type === "state") { renderState(event.data.state as HistoryState); }
});
renderState(state);
api.postMessage({ action: "ready" });
