import { test } from 'node:test';
import assert from 'node:assert/strict';

import { frame, MIN_COLUMNS, SELECTABLE } from './dashboard.mjs';

const visible = (html) => html.replace(/<[^>]+>/g, '').replaceAll('&lt;', '<').replaceAll('&gt;', '>').replaceAll('&amp;', '&');

test('every line of every frame has the same width, so the boxes close', () => {
  for (const selected of SELECTABLE) {
    const widths = new Set(frame(selected).map((line) => [...visible(line)].length));
    assert.equal(widths.size, 1, `frame ${selected}: widths ${[...widths]}`);
  }
});

test('a frame stretches to the columns it is given, never below its minimum', () => {
  for (const columns of [MIN_COLUMNS, 120, 151]) {
    const widths = new Set(frame('feat/login', columns).map((line) => [...visible(line)].length));
    assert.deepEqual([...widths], [columns]);
  }
  assert.equal([...visible(frame('main', 40)[0])].length, MIN_COLUMNS);
});

test('the selected worktree carries the selection bar and its name heads the detail pane', () => {
  const lines = frame('feat/login').map(visible);
  assert.ok(lines.some((l) => l.includes('▌ feat/login')));
  assert.ok(!lines.some((l) => l.includes('▌ main ')));
  assert.ok(lines.some((l) => (l.split('││')[1] ?? '').startsWith(' feat/login ')));
});

test('only the main checkout is marked as the current directory in the detail pane', () => {
  assert.ok(frame('main').map(visible).some((l) => l.includes('● you are here │')));
  assert.ok(!frame('feat/search').map(visible).some((l) => l.includes('● you are here │')));
});
