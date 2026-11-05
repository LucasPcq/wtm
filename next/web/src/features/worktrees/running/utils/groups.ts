import { jobCounts, type Read } from '@/api/queries/commands'
import type { ListJob, RunState, RunupPlan } from '@/api/types.gen'
import { jobPlan } from '@/features/worktrees/utils/job-plan'
import type { Progress } from '@/features/worktrees/utils/progress'
import { underWay } from '@/features/worktrees/utils/run-words'

// a job the read lists, or one a start of this page is bringing up that the read does not list yet
export type RunningRow = { name: string } & ({ job: ListJob } | { said: string })

export type RunningGroup = {
  worktree: string
  profile: string | undefined
  counts: Record<RunState, number>
  starting: boolean
  rows: RunningRow[]
  // the shared services it holds, which run in the main checkout's group
  holds: string[]
}

// what needs the person first: a crash, then what comes up, then what runs. Stopped and exited jobs are not listed
// yet: their hidden row comes with the jobs read (LUC-345)
const order: Partial<Record<RunState, number>> = { crashed: 0, starting: 1, running: 2 }

// a claim on a shared service is listed where that service runs: the main checkout's group
export function groupsOf({
  read,
  progress,
  plan,
}: {
  read: Read<'list'>
  progress: Progress<'run-up'> | undefined
  plan: RunupPlan | undefined
}): RunningGroup[] {
  return read.items.flatMap(({ subject, detail }) => {
    const own = (detail?.jobs ?? []).filter((job) => job.hold !== 'claim')
    const unit = progress?.units.find((one) => one.subject === subject)
    // a job of the start says where it stands until the read lists it as the start left it: not yet brought up, or
    // listed stopped from a run before
    const coming = (unit?.phases ?? []).flatMap((run) => {
      const job = run.job && jobPlan(plan, subject, run.job)
      if (!run.job || !job || job.action === 'join') return []
      const now = own.find((one) => one.name === run.job)
      const stale = !now || now.state === 'stopped' || run.state === 'todo' || run.state === 'now'
      return stale && !(run.state === 'done' && job.kind === 'task')
        ? [{ name: run.job, said: underWay(run, job, progress?.cancelling !== undefined) }]
        : []
    })
    const listed = own
      .filter(({ name, state }) => state && order[state] !== undefined && !coming.some((row) => row.name === name))
      .toSorted((a, b) => (order[a.state ?? 'running'] ?? 0) - (order[b.state ?? 'running'] ?? 0))
    if (listed.length === 0 && coming.length === 0) return []

    return [
      {
        worktree: subject,
        profile: detail?.profile || (unit ? plan?.profile : undefined),
        counts: jobCounts(listed),
        starting: coming.length > 0,
        rows: [...listed.map((job) => ({ name: job.name, job })), ...coming],
        holds: (detail?.jobs ?? [])
          .filter((job) => job.hold === 'claim' && job.state === 'running')
          .map((job) => job.name),
      },
    ]
  })
}

export function groupLine({ profile, counts, starting }: RunningGroup) {
  return [
    profile && `profile ${profile}`,
    counts.running > 0 && `${counts.running} up`,
    counts.crashed > 0 && `${counts.crashed} crashed`,
    starting && 'starting',
  ]
    .filter(Boolean)
    .join(' · ')
}
