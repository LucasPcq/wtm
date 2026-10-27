import type { Read, ReadCommand } from '@/api/queries/commands'
import type { DomainEnvelope, DomainFollowUp, ListDetail, StatusDetail, TreeDetail } from '@/api/types.gen'

export const listDetail = (branch: string, over: Partial<ListDetail> = {}): ListDetail => ({
  actions: [],
  active: false,
  ahead: 0,
  behind: 0,
  branch,
  changes: { changed: 0, untracked: 0 },
  commits_ahead: 0,
  condition: over.dirty ? 'changed' : 'clean',
  created_at: '2026-10-25T11:00:00Z',
  dirty: false,
  finished: null,
  last_commit_at: '2026-10-25T11:50:00Z',
  locked: false,
  main: false,
  merged: false,
  missing: false,
  needs_sync: false,
  origin: null,
  parent: 'main',
  path: `/acme.trees/${branch.replace('/', '-')}`,
  pr: null,
  services: [],
  urls: [],
  ...over,
})

export const treeDetail = (branch: string, over: Partial<TreeDetail> = {}): TreeDetail => ({
  actions: [],
  ahead: 0,
  behind: 0,
  branch,
  children: [],
  condition: 'clean',
  cycle: false,
  dirty: false,
  locked: false,
  main: false,
  missing: false,
  needs_sync: false,
  origin: null,
  parent: 'main',
  path: `/acme.trees/${branch.replace('/', '-')}`,
  pr: null,
  services: [],
  virtual: false,
  ...over,
})

export const statusDetail = (branch: string, over: Partial<StatusDetail> = {}): StatusDetail => ({
  base: { name: 'main', ahead: 0, behind: 0 },
  branch,
  changes: { deletions: 0, files: [], insertions: 0, modified: 0, staged: 0, untracked: 0 },
  children: [],
  commits: [],
  condition: 'clean',
  created_at: '2026-10-25T11:00:00Z',
  env: { declared: 0, files: [] },
  isolation: 'isolated',
  jobs: [],
  locked: false,
  main: false,
  missing: false,
  origin: null,
  parent: 'main',
  path: `/acme.trees/${branch.replace('/', '-')}`,
  problems: [],
  review: null,
  run: null,
  run_toml: 'absent',
  ...over,
})

export const followUp = (command: string, branch: string, over: Partial<DomainFollowUp> = {}): DomainFollowUp => ({
  command,
  answers: [{ path: 'worktree', values: [branch] }],
  code: `test.${command}`,
  message: command,
  ...over,
})

export const envelope = (command: string, details: { branch: string }[]): DomainEnvelope => ({
  v: 2,
  ok: true,
  command,
  follow_ups: [],
  warnings: [],
  items: details.map((detail) => ({ subject: detail.branch, status: 'done', detail: detail as never })),
})

export const read = <C extends ReadCommand>(command: C, details: Read<C>['items'][number]['detail'][]): Read<C> => ({
  v: 2,
  ok: true,
  command,
  plan: undefined,
  follow_ups: [],
  warnings: [],
  items: details.flatMap((detail) => (detail ? [{ subject: detail.branch, status: 'done', detail }] : [])),
})
