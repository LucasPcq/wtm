import { jobCounts, type Read } from '@/api/queries/commands'
import type { ListJob, RunState, RunupPlan } from '@/api/types.gen'
import { underWay } from '@/features/worktrees/run-up/utils/words'
import type { Progress } from '@/features/worktrees/utils/progress'

// a job the read lists, or one a start of this page is bringing up that the read does not list yet
export type RunningRow = { name: string } & ({ job: ListJob } | { said: string })

export type RunningGroup = {
  worktree: string
  profile: string | undefined
  counts: Record<RunState, number>
  starting: boolean
  rows: RunningRow[]
}

// what needs the person first: a crash, then what comes up, then what runs, then what is over
const order: Record<RunState, number> = { crashed: 0, starting: 1, running: 2, exited: 3, stopped: 4 }

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
  const planOf = (worktree: string, name: string) =>
    plan?.worktrees?.find((target) => target.worktree === worktree)?.jobs?.find((job) => job.name === name)

  return read.items.flatMap(({ subject, detail }) => {
    const jobs = (detail?.jobs ?? [])
      .filter((job) => job.hold !== 'claim')
      .toSorted((a, b) => order[a.state] - order[b.state])
    const unit = progress?.units.find((one) => one.subject === subject)
    const coming = (unit?.phases ?? []).flatMap((run) => {
      const job = run.job && planOf(subject, run.job)
      if (!run.job || !job || job.action === 'join' || jobs.some((listed) => listed.name === run.job)) return []
      return [{ name: run.job, said: underWay(run, job, progress?.cancelling !== undefined) }]
    })
    if (jobs.length === 0 && coming.length === 0) return []

    return [
      {
        worktree: subject,
        profile: jobs.find((job) => job.profile)?.profile ?? (unit ? plan?.profile : undefined),
        counts: jobCounts(jobs),
        starting: coming.length > 0,
        rows: [...jobs.map((job) => ({ name: job.name, job })), ...coming],
      },
    ]
  })
}

export function groupLine({ profile, counts, starting }: RunningGroup) {
  return [
    profile && `profile ${profile}`,
    counts.running > 0 && `${counts.running} up`,
    counts.crashed > 0 && `${counts.crashed} crashed`,
    counts.exited > 0 && `${counts.exited} exited`,
    starting && 'starting',
  ]
    .filter(Boolean)
    .join(' · ')
}
