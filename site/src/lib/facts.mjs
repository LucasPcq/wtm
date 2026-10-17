import { execFileSync } from 'node:child_process';

export const latestVersion = (tags) => tags.find((t) => /^v\d+\.\d+\.\d+$/.test(t))?.slice(1) ?? '';

const git = (...args) => execFileSync('git', args, { encoding: 'utf8' }).trim();

// Bundled into the page, import.meta.url no longer points into the repository, so
// the repository is found from the working directory instead of site.config.mjs.
export const releaseFacts = () => {
  const repo = git('rev-parse', '--show-toplevel');
  return { latest: latestVersion(git('-C', repo, 'tag', '--list', 'v*', '--sort=-v:refname').split('\n')) };
};
