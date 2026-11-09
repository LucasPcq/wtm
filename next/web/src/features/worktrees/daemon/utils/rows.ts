import type { DaemonrestartPlan, DaemonstopjobsPlan, RunHeldJob, RunHeldRepo, RunjobsStopJob } from '@/api/types.gen'

export type Row = { key: string; title: string; lines: string[]; warning?: string }
export type RepoRows = { repo: string; rows: Row[] }

export const repoName = (root: string) => root.split('/').filter(Boolean).at(-1) ?? 'no repository'

export const together = (names: string[]) =>
  names.length < 2 ? (names[0] ?? '') : `${names.slice(0, -1).join(', ')} and ${names.at(-1)}`

const holdersOf = (jobs: { held_by?: string[] | null }[]) => [...new Set(jobs.flatMap((job) => job.held_by ?? []))]

// the worktrees holding a row's shared services, which they lose: "feat/a loses it", "feat/a and fix/b lose them"
function losing(shared: { held_by?: string[] | null }[], after = '') {
  const holders = holdersOf(shared)
  if (holders.length === 0) return {}
  const verb = holders.length === 1 ? 'loses' : 'lose'
  return { warning: `${together(holders)} ${verb} ${shared.length === 1 ? 'it' : 'them'}${after}` }
}

// a claim is its shared job's, said on that job's row: its worktree is listed only for what it runs itself
function heldRows(repos: RunHeldRepo[] | null, after: string): RepoRows[] {
  return (repos ?? []).flatMap((repo) => {
    const rows = (repo.worktrees ?? []).flatMap((wt): Row[] => {
      const jobs = (wt.jobs ?? []).filter((job) => job.hold !== 'claim')
      if (jobs.length === 0) return []
      const shared = jobs.filter((job) => job.shared)
      const own = jobs.filter((job) => !job.shared)
      const lines = [own.map((job) => job.name).join(', '), shared.length > 0 && `${names(shared)} · shared`]
      return [
        {
          key: wt.path,
          title: wt.branch ?? wt.path,
          lines: lines.filter((line): line is string => Boolean(line)),
          ...losing(shared, after),
        },
      ]
    })
    return rows.length > 0 ? [{ repo: repoName(repo.root), rows }] : []
  })
}

const names = (jobs: { name: string }[]) => jobs.map((job) => job.name).join(', ')

export const restartRows = (plan: DaemonrestartPlan) => heldRows(plan.back, ' for the cut')

export const takenOverRows = (plan: DaemonrestartPlan): Row[] =>
  (plan.taken_over ?? []).flatMap((repo) =>
    (repo.worktrees ?? []).flatMap((wt) => {
      const stacks = (wt.jobs ?? []).filter((job) => job.hold === 'detached')
      if (stacks.length === 0) return []
      return [
        {
          key: wt.path,
          title: `${wt.branch ?? wt.path}'s stack`,
          lines: [`${names(stacks)} · started detached: it keeps running`],
        },
      ]
    }),
  )

export const notBackRows = (plan: DaemonrestartPlan): Row[] => {
  const why = (repos: RunHeldRepo[] | null, reason: string): Row[] =>
    (repos ?? []).flatMap((repo) =>
      (repo.worktrees ?? []).map((wt) => ({
        key: `${reason}-${wt.path}`,
        title: wt.branch ?? wt.path,
        lines: [`${names(wt.jobs ?? [])} · ${reason}`],
      })),
    )
  return [
    ...why(plan.tasks, 'a task, not run again'),
    ...why(plan.unrecorded, 'no record of it'),
    ...why(plan.gone, 'its worktree is gone'),
    ...why(plan.unheld, 'the worktrees holding it are gone'),
  ]
}

const stopCount = (jobs: RunjobsStopJob[]) => jobs.filter((job) => job.hold !== 'claim' && job.fate === 'stop').length

// the jobs every worktree stops: its own, its stacks by their stop command, a crash with nothing to stop, its shared
// services with whose hold goes; a hold on a shared service is said on that service's row
export function stopRows(plan: DaemonstopjobsPlan): RepoRows[] {
  return (plan.repos ?? []).flatMap((repo) => {
    const rows = (repo.worktrees ?? []).flatMap((target): Row[] => {
      const jobs = (target.jobs ?? []).filter((job) => job.hold !== 'claim')
      if (jobs.length === 0) return []
      const shared = jobs.filter((job) => job.shared_in && job.fate !== 'clear')
      const own = jobs.filter((job) => !job.shared_in && job.fate === 'stop' && job.hold !== 'detached')
      const stacks = jobs.filter((job) => job.hold === 'detached' && job.fate === 'stop')
      const crashed = jobs.filter((job) => job.fate === 'clear')
      const stack = `${own.length > 0 ? 'and its' : 'its'} stack: ${names(stacks)}, through its stop command`
      const ownLine = [names(own), stacks.length > 0 && stack].filter(Boolean).join(own.length > 0 ? ', ' : '')
      const lines = [
        ownLine,
        shared.length > 0 && `${names(shared)} · shared`,
        crashed.length > 0 && `${names(crashed)} crashed: nothing to stop`,
      ]
      return [
        {
          key: target.path,
          title: target.branch ?? target.path,
          lines: lines.filter((line): line is string => Boolean(line)),
          ...losing(shared),
        },
      ]
    })
    return rows.length > 0 ? [{ repo: repoName(repo.root), rows }] : []
  })
}

export const crashedTotal = (plan: DaemonstopjobsPlan) =>
  (plan.repos ?? [])
    .flatMap((repo) => repo.worktrees ?? [])
    .flatMap((target) => target.jobs ?? [])
    .filter((job) => job.fate === 'clear').length

export const stopTotal = (plan: DaemonstopjobsPlan) =>
  (plan.repos ?? []).reduce(
    (total, repo) => total + (repo.worktrees ?? []).reduce((n, t) => n + stopCount(t.jobs ?? []), 0),
    0,
  )

export function repoLine(repo: RunHeldRepo): string {
  const jobs: RunHeldJob[] = (repo.worktrees ?? []).flatMap((wt) => wt.jobs ?? []).filter((job) => job.hold !== 'claim')
  const stacks = jobs.filter((job) => job.hold === 'detached').length
  const own = jobs.length - stacks
  const counted = [
    own > 0 && (own === 1 ? '1 job' : `${own} jobs`),
    stacks > 0 && (stacks === 1 ? 'a stack' : `${stacks} stacks`),
  ].filter(Boolean)
  return counted.join(' and ')
}
