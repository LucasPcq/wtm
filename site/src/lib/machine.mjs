// What wtm writes for a worktree of the sandbox repository `acme`, following its
// rules (docs/guide/how-run-works.md, shared-services.md): the main checkout keeps
// the defaults, worktree n adds n × 10 to every port and names its own resources.
const REPO = 'acme';
const BLOCK = 10;

export const INITIAL = ['main', 'feat/login', 'feat/search'];
export const AGENTS = ['agent/checkout', 'agent/billing', 'agent/oauth'];

const esc = (s) => s.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');

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

const line = (e) => {
  const changed = e.was !== undefined && e.was !== e.value;
  const value = changed ? `<span class="p">${esc(e.value)}</span>` : esc(e.value);
  const note = changed ? `<span class="ln m"># main: ${esc(e.was)}</span>` : '';
  return `${note}<span class="ln"><span class="m">${e.key}=</span>${value}</span>`;
};

export const panel = (wt) =>
  wt.files.map((f) => `<div class="envfile"><div class="envfile-head"><span>${esc(f.path)}</span><span class="m">${f.by}</span></div><div class="envfile-body">${f.entries.map(line).join('')}</div></div>`).join('');
