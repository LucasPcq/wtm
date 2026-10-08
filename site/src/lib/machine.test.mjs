import { test } from 'node:test';
import assert from 'node:assert/strict';

import { lanes, MAX_AGENTS } from './machine.mjs';

test('one lane is the main checkout on the base ports', () => {
  const [main] = lanes(1);
  assert.equal(lanes(1).length, 1);
  assert.deepEqual(
    { branch: main.branch, web: main.web, api: main.api, compose: main.compose, database: main.database, url: main.url },
    { branch: 'main', web: 3000, api: 4000, compose: 'acme', database: 'app', url: 'http://localhost:3000' },
  );
});

test('a worktree shifts its ports by ten per ordinal and gets its own names', () => {
  const lane = lanes(2)[1];
  assert.equal(lane.branch, 'agent/fix-checkout');
  assert.equal(lane.slug, 'agent-fix-checkout');
  assert.equal(lane.web, 3010);
  assert.equal(lane.api, 4010);
  assert.equal(lane.compose, 'acme-agent-fix-checkout');
  assert.equal(lane.database, 'app_agent-fix-checkout');
  assert.equal(lane.url, 'http://web.agent-fix-checkout.acme.localhost');
});

test('the maximum has no duplicated port and the count is clamped', () => {
  const all = lanes(MAX_AGENTS);
  assert.equal(all.length, 12);
  const ports = all.flatMap((l) => [l.web, l.api]);
  assert.equal(new Set(ports).size, ports.length);
  assert.equal(lanes(40).length, 12);
  assert.equal(lanes(0).length, 1);
});

test('the hover event is a parseable job.started line', () => {
  const event = JSON.parse(lanes(2)[1].event);
  assert.equal(event.type, 'job.started');
  assert.equal(event.worktree.branch, 'agent/fix-checkout');
  assert.equal(event.job.url, 'http://web.agent-fix-checkout.acme.localhost');
});
