import { test } from 'node:test';
import assert from 'node:assert/strict';

import { monthYear, recentReleases } from './facts.mjs';

const changelog = `# Changelog

Intro.

## [Unreleased]

### Added

- something

## [0.29.2] - 2026-10-06

For scripts and agents: \`wtm events\` follows every repository.

### Added

- a bullet

## [0.29.1] - 2026-10-05

### Fixed

- no summary paragraph here

## [0.29.0] - 2026-10-04

The isolation release.
`;

test('recentReleases skips Unreleased and keeps order', () => {
  const releases = recentReleases(changelog, 2);
  assert.deepEqual(releases.map((r) => r.version), ['0.29.2', '0.29.1']);
  assert.equal(releases[0].date, '2026-10-06');
  assert.equal(releases[0].summary, 'For scripts and agents: `wtm events` follows every repository.');
});

test('a release without a summary paragraph gets an empty summary', () => {
  assert.equal(recentReleases(changelog, 3)[1].summary, '');
});

test('count larger than the list returns what exists', () => {
  assert.equal(recentReleases(changelog, 10).length, 3);
});

test('monthYear formats an ISO date in English', () => {
  assert.equal(monthYear('2026-04-01'), 'April 2026');
});
