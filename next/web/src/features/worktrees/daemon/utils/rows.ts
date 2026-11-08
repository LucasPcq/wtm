import type { DaemonrestartPlan, DaemonstopjobsPlan, RunHeldJob, RunHeldRepo, RunjobsStopJob } from '@/api/types.gen'

// a worktree as a dialog lists it: its name, a line per kind of job, and whose hold goes with it
export type Row = { key: string; title: string; lines: string[]; warning?: string }
export type RepoRows = { repo: string; rows: Row[] }

export const repoName = (root: string) => root.split('/').filter(Boolean).at(-1) ?? 'no repository'

export const together = (names: string[]) =>
  names.length < 2 ? (names[0] ?? '') : `${names.slice(0, -1).join(', ')} and ${names.at(-1)}`

const holdersOf = (jobs: { held_by?: string[] | null }[]) => [...new Set(jobs.flatMap((job) => job.held_by ?? []))]

// a claim is its shared job's, said on that job's row: its worktree is listed only for what it runs itself
function heldRows(repos: RunHeldRepo[] | null, losing: string): RepoRows[] {
  return (repos ?? []).flatMap((repo) => {
    const rows = (repo.worktrees ?? []).flatMap((wt): Row[] => {
      const jobs = (wt.jobs ?? []).filter((job) => job.hold !== 'claim')
      if (jobs.length === 0) return []
      const shared = jobs.filter((job) => job.shared)
      const own = jobs.filter((job) => !job.shared)
      const lines = [own.map((job) => job.name).join(', '), shared.length > 0 && `${names(shared)} · shared`]
      const holders = holdersOf(shared)
      return [
        {
          key: wt.path,
          title: wt.branch ?? wt.path,
          lines: lines.filter((line): line is string => Boolean(line)),
          ...(holders.length > 0 && { warning: `${together(holders)} ${losing}` }),
        },
      ]
    })
    return rows.length > 0 ? [{ repo: repoName(repo.root), rows }] : []
  })
}

const names = (jobs: { name: string }[]) => jobs.map((job) => job.name).join(', ')

export const restartRows = (plan: DaemonrestartPlan) => heldRows(plan.back, 'lose them for the cut')

// the stacks a restart takes over, one row per worktree's
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

// what a restart does not bring back, each with why
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
      const ownLine = [names(own), stacks.length > 0 && `and its stack: ${names(stacks)}, through its stop command`]
        .filter(Boolean)
        .join(own.length > 0 ? ', ' : '')
      const lines = [
        ownLine,
        shared.length > 0 && `${names(shared)} · shared`,
        crashed.length > 0 && `${names(crashed)} crashed: nothing to stop`,
      ]
      const holders = holdersOf(shared)
      return [
        {
          key: target.path,
          title: target.branch ?? target.path,
          lines: lines.filter((line): line is string => Boolean(line)),
          ...(holders.length > 0 && { warning: `${together(holders)} lose them` }),
        },
      ]
    })
    return rows.length > 0 ? [{ repo: repoName(repo.root), rows }] : []
  })
}

export const stopTotal = (plan: DaemonstopjobsPlan) =>
  (plan.repos ?? []).reduce(
    (total, repo) => total + (repo.worktrees ?? []).reduce((n, t) => n + stopCount(t.jobs ?? []), 0),
    0,
  )

// a repository's jobs as the panel says them: its jobs and its stacks counted, then their names
export function repoLine(repo: RunHeldRepo): string {
  const jobs: RunHeldJob[] = (repo.worktrees ?? []).flatMap((wt) => wt.jobs ?? []).filter((job) => job.hold !== 'claim')
  const stacks = jobs.filter((job) => job.hold === 'detached').length
  const own = jobs.length - stacks
  const counted = [
    own > 0 && (own === 1 ? '1 job' : `${own} jobs`),
    stacks > 0 && (stacks === 1 ? 'a stack' : `${stacks} stacks`),
  ].filter(Boolean)
  return `${counted.join(' and ')}: ${[...new Set(jobs.map((job) => job.name))].join(', ')}`
}
