import type { RunFate } from '@/api/types.gen'
import { useJobRun, useRun } from '@/features/worktrees/hooks/use-run'
import { useRunStartDialog } from '@/features/worktrees/hooks/use-run-start-dialog'
import type { JobDoing } from '@/features/worktrees/running/utils/job-words'

// what a run under way does to the job, as its row and its header say it: a start says whether it is a restart, as
// the run was asked; a shared service restarted through a worktree holding it restarts where it runs too
export function useJobDoing(repo: string, worktree: string, job: string, fate?: RunFate): JobDoing | undefined {
  const running = useJobRun(repo, worktree, job)
  const { ran } = useRunStartDialog(repo)
  const starting = useRun(repo, 'run-start').running
  const served = ran.plan?.worktrees?.[0]?.jobs?.[0]?.shared_in === worktree

  if (starting && ran.restart && ran.job === job && served) return 'restarting'
  if (running === 'run-start')
    return ran.worktree === worktree && ran.job === job && ran.restart ? 'restarting' : 'starting'
  if (running === 'run-stop') return fate === 'release' ? 'releasing' : 'stopping'
  return undefined
}
