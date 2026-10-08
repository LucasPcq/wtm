import { test } from 'node:test';
import assert from 'node:assert/strict';

import { cmd, out, render } from './term.mjs';

const lines = (html) => html.match(/<span class="ln[^"]*"[^>]*>.*?<\/span>(?=<span class="ln|$)/g) ?? [];
const RAIL = '<span class="p">┃</span>';

test('human output of a wtm command is framed by the violet rail, blank edges left out', () => {
  const html = render([cmd('wtm tree'), out(), out('  main'), out(), out('  └─ feat/a'), out(), cmd('ls')]);
  const [, top, main, gap, child, bottom] = lines(html);
  assert.ok(!top.includes(RAIL));
  assert.ok(main.includes(`${RAIL}  main`));
  assert.ok(gap.includes(RAIL));
  assert.ok(child.includes(RAIL));
  assert.ok(!bottom.includes(RAIL));
});

test('JSON output and other programs are never framed', () => {
  assert.ok(!render([cmd('wtm run up a --output json | jq -r .url'), out('http://a')]).includes(RAIL));
  assert.ok(!render([cmd('git worktree add ../a a'), out('Preparing worktree')]).includes(RAIL));
});
