// Connect to the real VS Code renderer launched by test-extension.mjs. The
// extension host remains free to handle webview messages while we click rows.
import { chromium, expect } from '@playwright/test';
const browser = await chromium.connectOverCDP('http://127.0.0.1:18742');
try {
  let frame;
  for (let attempt = 0; attempt < 100 && !frame; attempt++) {
    for (const context of browser.contexts()) {
      for (const page of context.pages()) {
        for (const candidate of page.frames()) {
          if (await candidate.locator('#table').count()) { frame = candidate; break; }
        }
      }
    }
    if (!frame) { await new Promise(resolve => setTimeout(resolve, 100)); }
  }
  if (!frame) { throw new Error('History webview was not found in the VS Code renderer'); }
  if (process.argv[2] === 'unpin') {
    await frame.locator('#pin').click();
    await expect(frame.locator('#pin')).toHaveAttribute('aria-pressed', 'false');
  } else {
    await expect(frame.locator('.commit')).toHaveCount(3);
    await frame.locator('.commit').first().click();
    await expect(frame.locator('.selected')).toHaveCount(1);
    await frame.locator('#table').press('ArrowDown');
    await expect(frame.locator('.selected .subject')).toHaveText('rename');
    await frame.locator('#table').press('Enter');
    if (process.env.GHIST_TEST_REVIEWS) {
      await expect(frame.locator('.review-link')).toHaveText('PR #42');
    }
    await frame.locator('#pin').click();
    await expect(frame.locator('#pin')).toHaveAttribute('aria-pressed', 'true');
    console.log('Real VS Code webview click, keyboard preview and pin passed');
  }
} finally { await browser.close(); }
