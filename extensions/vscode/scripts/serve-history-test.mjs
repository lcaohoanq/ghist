import { createServer } from 'node:http';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { extensionRoot } from './build-backend.mjs';
import path from 'node:path';
const require = createRequire(import.meta.url);
const { historyHtml } = require('../out/historyHtml.js');
const origin = 'http://127.0.0.1:18741';
createServer((request, response) => {
  const route = request.url.split('?')[0];
  const assets = {
    '/history.css': ['text/css', path.join(extensionRoot, 'media/history.css')],
    '/history.js': ['text/javascript', path.join(extensionRoot, 'out/client/webview/history.js')],
    '/theme.css': ['text/css', path.join(extensionRoot, 'tests/browser/theme.css')],
  };
  if (route === '/') {
    response.setHeader('Content-Type', 'text/html');
    response.end(historyHtml(`${origin}/history.css`, `${origin}/history.js`, origin, 'test-only-nonce')
      .replace('</head>', `<link rel="stylesheet" href="${origin}/theme.css"></head>`));
  } else if (assets[route]) {
    response.setHeader('Content-Type', assets[route][0]); response.end(readFileSync(assets[route][1]));
  } else { response.writeHead(404); response.end(); }
}).listen(18741, '127.0.0.1');
