import type { Read } from '@/api/queries/commands'
import type { RunpsCounts, RunpsJob, RunState, RunupPlan } from '@/api/types.gen'
import { jobPlan } from '@/features/worktrees/utils/job-plan'
import type { Progress } from '@/features/worktrees/utils/progress'
import { underWay } from '@/features/worktrees/utils/run-words'

// a job the read lists, or one a start of this page is bringing up that the read does not list yet
export type RunningRow = { name: string } & ({ job: RunpsJob } | { said: string })

// a worktree's jobs, or the repository's shared services (`shared`), which run in `worktree` too: the main checkout; a
// group is keyed by its repository and path (`shared` for the shared services), never by its branch, which `shared`
// or a filter's word could be
export type RunningGroup = {
  key: string
  worktree: string
  path: string | undefined
  shared: boolean
  profile: string | undefined
  counts: RunpsCounts
  starting: boolean
  rows: RunningRow[]
  ended: RunpsJob[]
}

// what has ended is set apart: the group closes on a quiet row that says it
export const endedStates: readonly RunState[] = ['exited', 'stopped', 'not_started']
export const hasEnded = (job: RunpsJob) => endedStates.includes(job.state)

// the read's groups in its order, crashes first; the rows of a start of this page join their worktree's group
export function groupsOf({
  read,
  progress,
  plan,
}: {
  read: Read<'run-ps'>
  progress: Progress<'run-up'> | undefined
  plan: RunupPlan | undefined
}): RunningGroup[] {
  const groups = read.items.flatMap(({ detail }) => {
    if (!detail) return []
    const worktree = detail.branch || detail.path
    const jobs = detail.jobs ?? []
    const coming = detail.shared ? [] : comingOf(worktree, jobs, progress, plan)
    const listed = jobs.filter((job) => !hasEnded(job) && !coming.some((row) => row.name === job.name))

    return [
      {
        key: detail.shared ? 'shared' : `${detail.repository}:${detail.path}`,
        worktree,
        path: detail.path,
        shared: detail.shared,
        profile: detail.profile || undefined,
        counts: detail.counts,
        starting: coming.length > 0,
        rows: [...listed.map((job) => ({ name: job.name, job })), ...coming],
        ended: jobs.filter((job) => hasEnded(job) && !coming.some((row) => row.name === job.name)),
      },
    ]
  })

  // a worktree the start brings up before the read lists any of its jobs
  const fresh = (progress?.units ?? []).flatMap((unit) => {
    if (groups.some((group) => group.worktree === unit.subject)) return []
    const coming = comingOf(unit.subject, [], progress, plan)
    return coming.length === 0
      ? []
      : [
          {
            // no path is known before the read lists it: a key no branch, nor the Pinned fold, can spell
            key: `fresh:${unit.subject}`,
            worktree: unit.subject,
            path: undefined,
            shared: false,
            profile: plan?.profile,
            counts: { running: 0, starting: 0, crashed: 0, exited: 0, stopped: 0, not_started: 0, holders: 0 },
            starting: true,
            rows: coming,
            ended: [],
          },
        ]
  })
  return [...groups, ...fresh]
}

// a job of the start says where it stands until the read lists it as the start left it: not yet brought up, or
// listed ended from a run before
function comingOf(
  worktree: string,
  jobs: RunpsJob[],
  progress: Progress<'run-up'> | undefined,
  plan: RunupPlan | undefined,
): RunningRow[] {
  const unit = progress?.units.find((one) => one.subject === worktree)
  return (unit?.phases ?? []).flatMap((run) => {
    const job = run.job && jobPlan(plan, worktree, run.job)
    if (!run.job || !job || job.action === 'join') return []
    const now = jobs.find((one) => one.name === run.job)
    const stale = !now || hasEnded(now) || run.state === 'todo' || run.state === 'now'
    return stale && !(run.state === 'done' && job.kind === 'task')
      ? [{ name: run.job, said: underWay(run, job, progress?.cancelling !== undefined) }]
      : []
  })
}

// the job the address names, shown or ended, so that the panel stays on it
export function groupJob(groups: RunningGroup[], worktree: string | undefined, job: string | undefined) {
  for (const group of groups) {
    if (group.worktree !== worktree) continue
    const shown = group.rows.find((row) => 'job' in row && (job === undefined || row.name === job))
    if (shown && 'job' in shown) return { group, job: shown.job }
    const ended = group.ended.find((one) => one.name === job)
    if (ended) return { group, job: ended }
  }
  return undefined
}
