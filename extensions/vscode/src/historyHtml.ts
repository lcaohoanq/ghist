// Only trusted extension-owned URLs are interpolated. Git data arrives through
// postMessage and is always rendered with textContent in the client.
export function historyHtml(css: string, script: string, source: string, nonce: string): string {
  const escape = (value: string) => value.replace(/[&"<>]/g, c => ({ "&": "&amp;", '"': "&quot;", "<": "&lt;", ">": "&gt;" }[c]!));
  return `<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src ${escape(source)}; script-src 'nonce-${escape(nonce)}'; img-src ${escape(source)}; base-uri 'none'; form-action 'none';">
<link rel="stylesheet" href="${escape(css)}"><title>ghist History</title></head>
<body>
<header>
  <div class="heading"><svg class="brand" viewBox="0 0 24 24" aria-hidden="true"><path d="M6 3v18M6 7h8a4 4 0 0 1 4 4v6"/><circle cx="6" cy="7" r="3"/><circle cx="6" cy="17" r="3"/><circle cx="18" cy="17" r="3"/></svg><strong>ghist History</strong><span id="repository"></span><span id="loading" role="status" hidden>Loading…</span></div>
  <div class="file-heading"><span id="file" title="">Choose a file to explore its history</span><span id="summary"></span></div>
  <div class="toolbar" role="toolbar" aria-label="History actions">
    <button id="choose" title="Choose a working file">Choose file</button>
    <button id="pin" aria-pressed="false" title="Pin this file; stop following the active editor">Pin file</button>
    <button id="refresh" title="Reload committed history">Refresh</button>
    <div class="navigation"><button id="newer" title="Select newer commit">↑ Newer</button><button id="older" title="Select older commit">↓ Older</button><button id="open-file" title="Open the complete file at the selected commit">Open file</button></div>
    <input id="search" type="search" placeholder="Search message, author or hash" aria-label="Search history" autocomplete="off" spellcheck="false">
  </div>
</header>
<div id="notice" role="status" hidden><span id="notice-text"></span><button id="retry">Retry</button></div>
<div id="empty" hidden><h2 id="empty-title"></h2><p id="empty-detail"></p><button id="empty-action">Choose file</button></div>
<div id="table" role="grid" aria-label="File history" aria-colcount="5" tabindex="0">
  <div class="columns" role="row"><span role="columnheader">Timeline</span><span role="columnheader">Message</span><span role="columnheader" class="author">Author</span><span role="columnheader">Time</span><span role="columnheader" class="hash">Hash</span></div>
  <div id="viewport"><div id="rows" role="rowgroup"></div></div>
</div>
<footer><span id="results"></span><span id="review-status" role="status"></span><span>↑ ↓ Select · Enter Preview · Ctrl/Cmd+F Search</span></footer>
<script type="module" nonce="${escape(nonce)}" src="${escape(script)}"></script>
</body></html>`;
}
