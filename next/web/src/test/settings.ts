import type { ConfigshowDetail } from '@/api/types.gen'

// acme's config.toml as config show reads it: the folder and the branch at their default, .env from the template, two
// create hooks, one clean hook, the parent's .env remembered
export const settingsDetail = (over: Partial<ConfigshowDetail> = {}): ConfigshowDetail => ({
  digest: 'd1',
  newer: false,
  settings: {
    fields: [
      { kind: 'text', path: 'base_path', section: 'worktrees', origin: 'config', value: '../.trees' },
      {
        kind: 'select',
        path: 'base_branch',
        section: 'worktrees',
        origin: 'config',
        value: 'main',
        options: [{ value: 'main' }, { value: 'develop' }],
        rememberable: false,
        remember: false,
        refreshable: false,
        closed: false,
      },
      {
        kind: 'select',
        path: 'env_strategy',
        section: 'env_files',
        origin: 'config',
        value: 'example',
        options: [{ value: 'example' }, { value: 'main' }, { value: 'parent' }, { value: 'none' }],
        rememberable: false,
        remember: false,
        refreshable: false,
        closed: true,
      },
      {
        kind: 'multi_select',
        path: 'env_files',
        section: 'env_files',
        origin: 'config',
        picked: ['apps/api/.env', 'apps/web/.env'],
        options: [{ value: 'apps/api/.env' }, { value: 'apps/web/.env' }],
      },
      {
        kind: 'hooks',
        path: 'on_create',
        section: 'hooks',
        origin: 'config',
        entries: [{ cmd: 'pnpm install' }, { cmd: 'pnpm db:generate', cwd: 'packages/db' }],
        found: [],
        dirs: ['packages/db'],
      },
      {
        kind: 'hooks',
        path: 'on_clean',
        section: 'hooks',
        origin: 'config',
        entries: [{ cmd: 'docker compose down' }],
        found: [],
        dirs: [],
      },
    ],
    problems: [],
    sections: [
      { id: 'worktrees', state: 'answered' },
      { id: 'env_files', state: 'answered' },
      { id: 'hooks', state: 'answered' },
    ],
  },
  defaults: [
    { path: 'base_path', value: '../.trees', in_use: true },
    { path: 'base_branch', value: 'main', in_use: true },
    { path: 'env_strategy', value: 'example', in_use: true },
  ],
  remembered: [{ question: 'env_strategy', value: 'parent', entry: 'env_strategy=parent' }],
  ...over,
})
