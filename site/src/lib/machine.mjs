// The numbers follow wtm's rules (docs/guide/how-run-works.md, addressing.md,
// shared-services.md) so the figure stays true: base ports on main, +10 per ordinal.
const REPO = 'acme';
const WEB = 3000;
const API = 4000;
const BLOCK = 10;
const TASKS = ['fix-checkout', 'add-search', 'oauth-login', 'dark-mode', 'rate-limit', 'i18n', 'csv-export', 'retry-jobs', 'audit-log', 'cache-warmup', 'billing-v2'];

export const MAX_AGENTS = TASKS.length + 1;

const lane = (ordinal) => {
  const branch = ordinal === 0 ? 'main' : `agent/${TASKS[ordinal - 1]}`;
  const slug = branch.replaceAll('/', '-');
  const web = WEB + ordinal * BLOCK;
  const url = ordinal === 0 ? `http://localhost:${web}` : `http://web.${slug}.${REPO}.localhost`;
  const event = JSON.stringify({
    v: 1,
    type: 'job.started',
    worktree: { branch, path: ordinal === 0 ? `/code/${REPO}` : `/code/.trees/${slug}` },
    job: { name: 'web', kind: 'service', url },
  });
  return {
    branch,
    slug,
    ordinal,
    web,
    api: API + ordinal * BLOCK,
    compose: ordinal === 0 ? REPO : `${REPO}-${slug}`,
    database: ordinal === 0 ? 'app' : `app_${slug}`,
    url,
    event,
  };
};

export const lanes = (count) => {
  const clamped = Math.min(Math.max(Math.trunc(count) || 1, 1), MAX_AGENTS);
  return Array.from({ length: clamped }, (_, ordinal) => lane(ordinal));
};
