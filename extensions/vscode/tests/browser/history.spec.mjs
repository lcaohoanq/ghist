import { test, expect } from '@playwright/test';
const versions = (count = 40) => Array.from({ length: count }, (_, i) => ({
  hash: i.toString(16).padStart(40, 'a'), shortHash: i.toString(16).padStart(7, 'a'),
  subject: `Commit ${i}`, author: `Author ${i}`, email: 'test@example.com', date: '2026-10-14T09:00:00Z',
  parent: 'b'.repeat(40), merge: i === 3, beforePath: i === 0 ? '' : 'old.ts', afterPath: i === 1 ? '' : 'file.ts',
  change: i === 0 ? 'A' : i === 1 ? 'D' : i === 2 ? 'R100' : 'M',
}));
const state = (commits = versions()) => ({ context: 1, target: '/workspace/repository/file.ts', pinned: false, loading: false,
  history: { root: '/workspace/repository', path: 'file.ts', head: 'b'.repeat(40), versions: commits } });
async function load(page, data = state()) {
  await page.addInitScript(() => {
    window.actions = [];
    window.acquireVsCodeApi = () => ({ getState: () => undefined, setState: () => {}, postMessage: message => {
      window.actions.push(message);
      if (['select', 'diff'].includes(message.action)) {
        window.hostState.selectedHash = message.hash;
        window.postMessage({ type: 'state', state: window.hostState }, '*');
      }
    } });
  });
  await page.goto('/');
  await page.evaluate(data => { window.hostState = data; window.postMessage({ type: 'state', state: data }, '*'); }, data);
  await expect(page.locator('#loading')).toBeHidden();
}

test('keyboard selects, Enter previews, click keeps focus, and toolbar sends only identifiers', async ({ page }) => {
  await load(page);
  await page.locator('#table').focus(); await page.keyboard.press('ArrowDown');
  await expect(page.locator('.selected')).toHaveAttribute('data-hash', versions()[0].hash);
  expect(await page.evaluate(() => window.actions.filter(a => a.action === 'diff'))).toHaveLength(0);
  await page.keyboard.press('ArrowDown'); await page.keyboard.press('Enter');
  expect(await page.evaluate(() => window.actions.at(-1))).toEqual({ action: 'diff', hash: versions()[1].hash, context: 1 });
  await expect(page.locator('#open-file')).toBeDisabled();
  await page.locator('.commit').nth(2).click(); await expect(page.locator('#table')).toBeFocused();
  await page.locator('#open-file').click();
  expect(await page.evaluate(() => window.actions.at(-1))).toEqual({ action: 'file', hash: versions()[2].hash, context: 1 });
  await page.locator('#older').click(); await expect(page.locator('.selected')).toHaveAttribute('data-hash', versions()[3].hash);
  await page.locator('#newer').click(); await expect(page.locator('.selected')).toHaveAttribute('data-hash', versions()[2].hash);
  for (const action of ['pin', 'refresh', 'choose']) {
    await page.locator(`#${action}`).click(); expect((await page.evaluate(() => window.actions.at(-1))).action).toBe(action);
  }
});

test('local search covers message, author and hash without host requests', async ({ page }) => {
  await load(page);
  await page.keyboard.press('Control+f'); await expect(page.locator('#search')).toBeFocused();
  const before = await page.evaluate(() => window.actions.length);
  for (const query of ['Commit 12', 'Author 12', versions()[12].hash]) {
    await page.locator('#search').fill(query); await expect(page.locator('.commit')).toHaveCount(1);
    await expect(page.locator('.subject')).toHaveText('Commit 12');
  }
  expect(await page.evaluate(() => window.actions.length)).toBe(before);
  await page.locator('#search').fill('no match'); await expect(page.locator('#empty-title')).toHaveText('No matching commits');
  await page.locator('#empty-action').click(); await expect(page.locator('#table')).toBeVisible();
});

test('shows PR/MR links and sends only validated row identifiers to the host', async ({ page }) => {
  const data = state();
  data.reviews = {
    [data.history.versions[0].hash]: [
      { provider: 'github', number: 12, url: 'https://github.com/acme/repo/pull/12' },
      { provider: 'gitlab', number: 7, url: 'https://gitlab.com/acme/repo/-/merge_requests/7' },
    ],
  };
  await load(page, data);
  await expect(page.locator('.review-link')).toHaveCount(2);
  await expect(page.locator('.review-link').first()).toHaveText('PR #12');
  await expect(page.locator('.review-link').last()).toHaveText('MR !7');
  await page.locator('.review-link').first().click();
  expect(await page.evaluate(() => window.actions.at(-1))).toEqual({
    action: 'openReview', hash: data.history.versions[0].hash, index: 0, context: 1,
  });
  expect(await page.evaluate(() => window.actions.filter(action => action.action === 'diff'))).toHaveLength(0);
});

test('long and untrusted data is text, columns align and narrow layouts expose details through tooltips', async ({ page }) => {
  const commits = versions(); commits[0].subject = '<img src=x onerror=alert(1)>' + ' long'.repeat(200);
  commits[0].author = 'Very long author name '.repeat(20);
  await load(page, state(commits));
  expect(await page.locator('.subject').first().textContent()).toBe(commits[0].subject);
  await expect(page.locator('#rows img')).toHaveCount(0);
  await expect(page.locator('.badge.added')).toHaveText('Added');
  await expect(page.locator('.badge.deleted')).toHaveText('Deleted');
  await expect(page.locator('.badge.renamed')).toHaveText('Renamed');
  await expect(page.locator('.badge.merge')).toHaveAttribute('title', /first parent/);
  const sizes = await page.locator('.commit').first().evaluate(row => ({ height: row.getBoundingClientRect().height, subject: row.querySelector('.subject').getBoundingClientRect().right, author: row.querySelector('.author').getBoundingClientRect().left }));
  expect(sizes.height).toBe(36); expect(sizes.subject).toBeLessThan(sizes.author);
  await page.setViewportSize({ width: 600, height: 700 });
  await expect(page.locator('.commit .author').first()).toBeHidden(); await expect(page.locator('.commit .hash').first()).toBeHidden();
  await expect(page.locator('.commit').first()).toHaveAttribute('title', new RegExp(commits[0].hash));
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(600);
});

test('empty, loading, no history, refresh and errors provide appropriate actions', async ({ page }) => {
  await load(page, { context: 0, pinned: false, loading: false });
  await expect(page.locator('#empty-title')).toHaveText('Explore a file’s history');
  const update = data => page.evaluate(data => window.postMessage({ type: 'state', state: data }, '*'), data);
  await update({ context: 1, target: '/repo/a', pinned: false, loading: true });
  await expect(page.locator('#loading')).toBeVisible(); await expect(page.locator('#empty-title')).toHaveText('Loading history…');
  await update({ context: 1, target: '/repo/a', pinned: false, loading: false, empty: true });
  await expect(page.locator('#empty-title')).toHaveText('No committed history');
  await expect(page.locator('#empty-action')).toHaveText('Refresh history');
  await update({ ...state(), loading: true, selectedHash: versions()[2].hash, pinned: true });
  await expect(page.locator('#table')).toBeVisible(); await expect(page.locator('#pin')).toHaveAttribute('aria-pressed', 'true');
  await update({ ...state(), error: 'Backend failed <script>', selectedHash: versions()[2].hash });
  await expect(page.locator('#notice-text')).toHaveText('Backend failed <script>'); await expect(page.locator('.selected')).toBeVisible();
  await page.locator('#retry').click(); expect((await page.evaluate(() => window.actions.at(-1))).action).toBe('refresh');
  await update({ context: 2, target: '/repo/b', pinned: false, loading: false, error: 'Git unavailable' });
  await expect(page.locator('#empty-title')).toHaveText('History could not be loaded');
});

for (const theme of ['vscode-dark', 'vscode-light', 'vscode-high-contrast', 'vscode-high-contrast-light']) {
  test(`theme ${theme} uses editor tokens and visible selection`, async ({ page }, testInfo) => {
    await load(page); await page.evaluate(theme => { document.body.className = theme; }, theme);
    const stroke = await page.locator('.timeline svg').nth(1).evaluate(svg => getComputedStyle(svg).stroke);
    expect(stroke).not.toBe('none');
    expect(stroke).not.toBe('rgba(0, 0, 0, 0)');
    await page.locator('.commit').first().click();
    const colors = await page.locator('.selected').evaluate(row => ({ foreground: getComputedStyle(row).color, background: getComputedStyle(row).backgroundColor, outline: getComputedStyle(row).outlineStyle }));
    expect(colors.foreground).not.toBe(colors.background); expect(colors.background).not.toBe('rgba(0, 0, 0, 0)');
    if (theme.includes('high-contrast')) { expect(colors.outline).toBe('solid'); }
    await page.screenshot({ path: testInfo.outputPath(`${theme}.png`) });
  });
}

test('10,000 commits stay virtualized and keyboard navigation scrolls to selection', async ({ page }) => {
  await load(page, state(versions(10000)));
  await expect(page.locator('#summary')).toContainText('10,000 commits');
  expect(await page.locator('.commit').count()).toBeLessThan(60);
  await page.locator('#viewport').evaluate(node => { node.scrollTop = node.scrollHeight; });
  await expect(page.locator('.subject').last()).toHaveText('Commit 9999');
  expect(await page.locator('.commit').count()).toBeLessThan(60);
  await page.locator('.commit').last().click(); await page.keyboard.press('ArrowUp');
  await expect(page.locator('.selected .subject')).toHaveText('Commit 9998');
  await page.evaluate(() => window.postMessage({ type: 'state', state: { ...window.hostState, selectedHash: window.hostState.history.versions[0].hash } }, '*'));
  await page.locator('#table').focus(); await page.keyboard.press('ArrowDown');
  await expect(page.locator('.selected .subject')).toHaveText('Commit 1');
  expect(await page.locator('#viewport').evaluate(node => node.scrollTop)).toBeLessThan(100);
});
