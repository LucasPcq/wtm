import { test } from 'node:test';
import assert from 'node:assert/strict';

import { latestVersion } from './facts.mjs';

test('the latest version is the first release tag, without its v', () => {
  assert.equal(latestVersion(['v0.29.2', 'v0.29.1', 'v0.28.0']), '0.29.2');
});

test('pre-release and foreign tags are skipped', () => {
  assert.equal(latestVersion(['v0.30.0-rc.1', 'nightly', 'v0.29.2']), '0.29.2');
});

test('no release tag gives an empty version', () => {
  assert.equal(latestVersion([]), '');
});
