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

const git = (...args) => execFileSync('git', args, { encoding: 'utf8' }).trim();

// Bundled into the page, import.meta.url no longer points into the repository, so
// the repository is found from the working directory instead of site.config.mjs.
export const releaseFacts = () => {
  const repo = git('rev-parse', '--show-toplevel');
  const tags = git('-C', repo, 'tag', '--list', 'v*', '--sort=-v:refname').split('\n').filter((t) => /^v\d+\.\d+\.\d+$/.test(t));
  const firstCommit = git('-C', repo, 'log', '--reverse', '--format=%ad', '--date=short').split('\n')[0];
  return {
    releases: tags.length,
    latest: tags[0]?.slice(1) ?? '',
    since: monthYear(firstCommit),
    recent: recentReleases(readFileSync(join(repo, 'CHANGELOG.md'), 'utf8'), 3),
  };
};
