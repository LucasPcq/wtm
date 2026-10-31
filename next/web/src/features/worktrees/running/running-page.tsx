import { useQuery } from '@tanstack/react-query'
import { Activity, Play } from 'lucide-react'
import { readQuery } from '@/api/queries/commands'
import { Centered } from '@/components/centered'
import { PageHeader } from '@/components/page-header'
import { StatePage } from '@/components/state-page'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ReadFailed } from '@/features/worktrees/components/read-failed'
import { ReasonButton } from '@/features/worktrees/components/reason-button'
import { clockEvery, useRun } from '@/features/worktrees/hooks/use-run'
import { openRunUp, useRunUpDialog } from '@/features/worktrees/hooks/use-run-up-dialog'
import { RunUpDialog } from '@/features/worktrees/run-up/run-up-dialog'
import { RunGroup } from '@/features/worktrees/running/components/run-group'
import { groupsOf } from '@/features/worktrees/running/utils/groups'
import { plural } from '@/lib/plural'

// shortcut: the jobs come from list's read, counted here; the jobs read of its own (with its counts, its order and the
// shared services' group) replaces it
export function RunningPage({ repo }: { repo: string }) {
  const list = useQuery(readQuery(repo, 'list'))
  const { progress, running } = useRun(repo, 'run-up')
  const plan = useRunUpDialog((dialog) => dialog.ran.plan)
  const now = useQuery({ queryKey: ['now'], queryFn: () => Date.now(), refetchInterval: clockEvery }).data ?? 0

  // past its end, a start's progress holds until the read after it lands: its groups would vanish in between
  const overlay = running || list.isFetching ? progress : undefined
  const groups = list.data ? groupsOf({ read: list.data, progress: overlay, plan }) : []
  const up = groups.reduce((count, group) => count + group.counts.running, 0)
  const crashed = groups.reduce((count, group) => count + group.counts.crashed, 0)

  const description = () => {
    if (!list.data) return 'Reading the jobs…'
    if (groups.length === 0) return 'Nothing running'
    const where = `${plural(up, 'job')} up in ${plural(groups.length, 'worktree')}`
    return crashed > 0 ? `${where} · ${crashed} crashed` : where
  }

  const start = (
    <Button onClick={() => openRunUp(repo)}>
      <Play />
      Start a profile
    </Button>
  )

  const body = () => {
    if (list.isError) return <ReadFailed message={list.error.message} />
    if (!list.data) return <Skeletons />
    if (groups.length === 0)
      return (
        <Centered>
          <StatePage icon={<Activity />} title="Nothing is running" action={start}>
            Start a profile to run its jobs in the worktrees you pick.
          </StatePage>
        </Centered>
      )
    return (
      <div className="flex flex-col divide-y px-8 py-2">
        {groups.map((group) => (
          <RunGroup key={group.worktree} repo={repo} group={group} now={now} />
        ))}
      </div>
    )
  }

  return (
    <>
      <PageHeader
        title="Running"
        description={description()}
        action={
          <div className="flex gap-2">
            <ReasonButton disabled why="Not in the web yet" onClick={() => {}} variant="outline">
              Stop jobs
            </ReasonButton>
            {start}
          </div>
        }
      />
      <RunUpDialog repo={repo} />
      <div className="min-h-0 flex-1 overflow-y-auto" aria-busy={!list.data}>
        {body()}
      </div>
    </>
  )
}

function Skeletons() {
  return (
    <div className="flex flex-col gap-6 px-8 py-6">
      {[0, 1, 2].map((group) => (
        <div key={group} className="flex flex-col gap-2.5">
          <Skeleton className="h-3.5 w-48" />
          <Skeleton className="h-3 w-64" />
          <Skeleton className="h-3 w-80" />
          <Skeleton className="h-3 w-72" />
        </div>
      ))}
    </div>
  )
}
