import type { Read, ReadCommand } from '@/api/queries/commands'
import type {
  DomainEnvelope,
  DomainFollowUp,
  ListDetail,
  RepositoryOffer,
  RunpsCounts,
  RunpsDetail,
  RunpsJob,
  StatusDetail,
  TreeDetail,
  WorktreeAction,
  WorktreeUnoffered,
} from '@/api/types.gen'

const commands: Record<WorktreeAction, string> = {
  go: 'go',
  open_pr: '',
  exec: 'exec',
  run_up: 'run-up',
  run_down: 'run-down',
  logs: 'run-logs',
  fast_forward: 'fast-forward',
  sync: 'sync',
  reparent: 'reparent',
  clean: 'clean',
}

export const offers = (
  worktree: string,
  refused: Partial<Record<WorktreeAction, WorktreeUnoffered>> = {},
): RepositoryOffer[] =>
  (Object.keys(commands) as WorktreeAction[]).map((action) => {
    const why = refused[action]
    if (why) return { action, follow_up: null, refused: why }
    return {
      action,
      follow_up:
        action === 'open_pr'
          ? { code: 'list.open_pr', message: 'Open the pull request', url: 'https://github.com/acme/acme/pull/1' }
          : followUp(commands[action], worktree, { code: `list.${action}` }),
      refused: null,
    }
  })

const mainOffers = (branch: string) =>
  offers(branch, {
    open_pr: 'no_pull_request',
    sync: 'main_worktree',
    reparent: 'main_worktree',
    clean: 'main_worktree',
  })

export const listDetail = (branch: string, over: Partial<ListDetail> = {}): ListDetail => ({
  actions: over.main ? mainOffers(branch) : offers(branch, { open_pr: 'no_pull_request' }),
  active: false,
  ahead: 0,
  behind: 0,
  branch,
  changes: { changed: 0, untracked: 0 },
  commits_ahead: 0,
  condition: over.dirty ? 'changed' : 'clean',
  created_at: '2026-10-25T11:00:00Z',
  jobs: [],
  profile: '',
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
  actions: over.main ? mainOffers(branch) : offers(branch, { open_pr: 'no_pull_request' }),
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
  parent_gone: false,
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

export const read = <C extends Exclude<ReadCommand, 'run-logs' | 'daemon-status' | 'config-show'>>(
  command: C,
  details: Read<C>['items'][number]['detail'][],
): Read<C> => ({
  v: 2,
  ok: true,
  command,
  plan: undefined,
  follow_ups: [],
  warnings: [],
  items: details.flatMap((detail) => (detail ? [{ subject: detail.branch, status: 'done', detail }] : [])),
})

// a read whose one item it could not read: the envelope answers, the item says why
export const unreadable = (command: string, subject: string): DomainEnvelope => ({
  ...envelope(command, []),
  ok: false,
  error: { kind: 'internal', code: `${command}.unreadable`, message: `couldn't read /acme.trees/${subject}` },
  items: [
    {
      subject,
      status: 'failed',
      error: { kind: 'internal', code: `${command}.unreadable`, message: `couldn't read /acme.trees/${subject}` },
      detail: {},
    },
  ],
})

const countsOf = (jobs: RunpsJob[]): RunpsCounts => {
  const counts: RunpsCounts = { running: 0, starting: 0, crashed: 0, exited: 0, stopped: 0, not_started: 0, holders: 0 }
  for (const job of jobs) counts[job.state] += 1
  return counts
}

export const psJob = (name: string, over: Partial<RunpsJob> = {}): RunpsJob => ({
  name,
  kind: 'service',
  state: 'running',
  hold: 'own',
  started_at: '2026-10-25T11:50:00Z',
  ...over,
})

// a group of run ps: a worktree's jobs, its counts counted from them
export const psGroup = (branch: string, jobs: RunpsJob[], over: Partial<RunpsDetail> = {}): RunpsDetail => ({
  repository: '/acme',
  shared: false,
  branch,
  path: branch === 'main' ? '/acme' : `/acme.trees/${branch.replace('/', '-')}`,
  profile: 'dev',
  counts: countsOf(jobs),
  jobs,
  ...over,
})

export const psEnvelope = (groups: RunpsDetail[]): DomainEnvelope => ({
  ...envelope('run-ps', []),
  items: groups.map((detail) => ({
    subject: detail.shared ? 'shared' : detail.branch,
    status: 'done',
    detail: detail as never,
  })),
})

// run ps of what list says: each worktree's own jobs, a crash first, its claims left out
export const psOf = (details: ListDetail[]): DomainEnvelope =>
  psEnvelope(
    details
      .flatMap((detail) => {
        const jobs = (detail.jobs ?? []).flatMap((job) =>
          job.hold === 'claim'
            ? []
            : [
                psJob(job.name, {
                  ...(job.kind && { kind: job.kind }),
                  state: job.state ?? 'running',
                  ...(job.hold && { hold: job.hold }),
                  ...(job.url && { url: job.url }),
                  ...(job.exit_code !== undefined && { exit_code: job.exit_code }),
                  ...(job.started_at && { started_at: job.started_at }),
                }),
              ],
        )
        return jobs.length > 0
          ? [psGroup(detail.branch, jobs, { profile: detail.profile ?? '', path: detail.path })]
          : []
      })
      .toSorted((a, b) => Number(b.counts.crashed > 0) - Number(a.counts.crashed > 0)),
  )
