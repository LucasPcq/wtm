import { test } from 'node:test';
import assert from 'node:assert/strict';

import { AGENTS, INITIAL, panel, worktree } from './machine.mjs';

const entry = (wt, path, key) => wt.files.find((f) => f.path === path).entries.find((e) => e.key === key);

test('the main checkout keeps the project defaults and is compared to nothing', () => {
  const main = worktree('main', 0);
  assert.equal(entry(main, 'apps/web/.env', 'PORT').value, '3000');
  assert.equal(entry(main, 'apps/web/.env', 'API_URL').value, 'http://localhost:4000');
  assert.equal(entry(main, 'apps/api/.env', 'DATABASE_URL').value, 'postgresql://app:app@localhost:5432/app');
  assert.ok(main.files.flatMap((f) => f.entries).every((e) => e.was === undefined));
});

test('a worktree shifts its ports by ten per ordinal and names its own resources', () => {
  const wt = worktree('feat/login', 1);
  assert.deepEqual(entry(wt, 'apps/web/.env', 'PORT'), { key: 'PORT', value: '3010', was: '3000' });
  assert.equal(entry(wt, 'apps/web/.env', 'API_URL').value, 'http://api.feat-login.acme.localhost');
  assert.equal(entry(wt, 'apps/api/.env', 'PORT').value, '4010');
  assert.equal(entry(wt, 'apps/api/.env', 'DATABASE_URL').value, 'postgresql://app:app@localhost:5432/app_feat-login');
  const job = wt.files.find((f) => f.path.startsWith('environment')).entries;
  assert.deepEqual(Object.fromEntries(job.map((e) => [e.key, e.value])), {
    WTM_BRANCH: 'feat/login', WTM_WORKTREE: 'feat-login', WTM_ORDINAL: '1', WTM_PORT_OFFSET: '10', COMPOSE_PROJECT_NAME: 'acme-feat-login',
  });
});

test('no two worktrees on the page share a port', () => {
  const all = [...INITIAL, ...AGENTS].map((branch, ordinal) => worktree(branch, ordinal));
  const ports = all.flatMap((wt) => wt.files.flatMap((f) => f.entries.filter((e) => e.key === 'PORT').map((e) => e.value)));
  assert.equal(new Set(ports).size, ports.length);
});

test('a changed value is preceded by what main has, as a shell comment', () => {
  const html = panel(worktree('feat/login', 1));
  assert.match(html, /<span class="ln m"># main: 3000<\/span><span class="ln"><span class="m">PORT=<\/span><span class="p">3010<\/span>/);
  assert.doesNotMatch(panel(worktree('main', 0)), /# main:/);
});
