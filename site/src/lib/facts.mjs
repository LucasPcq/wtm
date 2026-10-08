import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const HEADING = /^## \[(\d+\.\d+\.\d+)\] - (\d{4}-\d{2}-\d{2})$/;

export const recentReleases = (markdown, count) => {
  const lines = markdown.split('\n');
  const releases = [];
  for (let i = 0; i < lines.length && releases.length < count; i++) {
    const match = lines[i].match(HEADING);
    if (!match) continue;
    const next = lines.slice(i + 1).find((line) => line.trim() !== '') ?? '';
    const summary = /^(#|-|\*)/.test(next) ? '' : next.trim();
    releases.push({ version: match[1], date: match[2], summary });
  }
  return releases;
};

export const monthYear = (iso) =>
  new Date(`${iso}T00:00:00Z`).toLocaleDateString('en-US', { month: 'long', year: 'numeric', timeZone: 'UTC' });

export const firstCommitDate = (repo) =>
  execFileSync('git', ['-C', repo, 'log', '--reverse', '--format=%ad', '--date=short'], { encoding: 'utf8' }).split('\n')[0];

export const releaseFacts = async () => {
  const { REPO, stableTags } = await import('../../site.config.mjs');
  return {
    releases: stableTags.length,
    latest: stableTags[0]?.slice(1) ?? '',
    since: monthYear(firstCommitDate(REPO)),
    recent: recentReleases(readFileSync(join(REPO, 'CHANGELOG.md'), 'utf8'), 3),
  };
};
