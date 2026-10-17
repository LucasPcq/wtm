// What wtm writes for a worktree of the sandbox repository `acme`, following its
// rules (docs/guide/ports-and-env.md, shared-services.md): the main checkout keeps
// the defaults, worktree n adds n × 10 to every port and names its own resources.
const REPO = 'acme';
const BLOCK = 10;

// The three branches the landing follows, in the order wtm created them.
export const LANES = [
  { branch: 'feat/login', who: 'you' },
  { branch: 'agent/checkout', who: 'claude' },
  { branch: 'agent/search', who: 'codex' },
];

export const worktree = (branch, ordinal) => {
  const slug = branch.replaceAll('/', '-');
  const main = ordinal === 0;
  const offset = ordinal * BLOCK;
  const vs = (value, mainValue) => (main ? { value } : { value, was: mainValue });
  const pair = (key, value, mainValue) => ({ key, ...vs(value, mainValue) });
  return {
    branch,
    slug,
    files: [
      {
        path: 'apps/web/.env',
        by: 'wtm create',
        entries: [
          pair('PORT', String(3000 + offset), '3000'),
          pair('API_URL', main ? 'http://localhost:4000' : `http://api.${slug}.${REPO}.localhost`, 'http://localhost:4000'),
        ],
      },
      {
        path: 'apps/api/.env',
        by: 'wtm create',
        entries: [
          pair('PORT', String(4000 + offset), '4000'),
          pair('DATABASE_URL', `postgresql://app:app@localhost:5432/${main ? 'app' : `app_${slug}`}`, 'postgresql://app:app@localhost:5432/app'),
        ],
      },
      {
        path: 'environment of every job',
        by: 'wtm run up',
        entries: [
          { key: 'WTM_BRANCH', value: branch },
          { key: 'WTM_WORKTREE', value: slug },
          { key: 'WTM_ORDINAL', value: String(ordinal) },
          { key: 'WTM_PORT_OFFSET', value: String(offset) },
          { key: 'COMPOSE_PROJECT_NAME', value: `${REPO}-${slug}` },
        ],
      },
    ],
  };
};

const value = (wt, path, key) => wt.files.find((f) => f.path === path).entries.find((e) => e.key === key).value;

// One worktree's identity as the landing shows it: its address, ports, Compose project and database.
export const card = (wt) => ({
  branch: wt.branch,
  url: `web.${wt.slug}.${REPO}.localhost`,
  webPort: value(wt, 'apps/web/.env', 'PORT'),
  apiPort: value(wt, 'apps/api/.env', 'PORT'),
  compose: value(wt, 'environment of every job', 'COMPOSE_PROJECT_NAME'),
  database: value(wt, 'apps/api/.env', 'DATABASE_URL').split('/').pop(),
});

export const lanes = () => LANES.map((lane, i) => ({ ...lane, ...card(worktree(lane.branch, i + 1)) }));
