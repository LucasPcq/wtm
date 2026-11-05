import { useQuery } from '@tanstack/react-query'
import { Activity, Play } from 'lucide-react'
import { readQuery } from '@/api/queries/commands'
import { reposQuery } from '@/api/queries/repos'
import { Centered } from '@/components/centered'
import { PageHeader } from '@/components/page-header'
import { StatePage } from '@/components/state-page'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ReadFailed } from '@/features/worktrees/components/read-failed'
import { ReasonButton } from '@/features/worktrees/components/reason-button'
import { clockEvery, useRun } from '@/features/worktrees/hooks/use-run'
import { openRunDown } from '@/features/worktrees/hooks/use-run-down-dialog'
import { openRunUp, useRunUpDialog } from '@/features/worktrees/hooks/use-run-up-dialog'
import { RunDownDialog } from '@/features/worktrees/run-down/run-down-dialog'
import { RunUpDialog } from '@/features/worktrees/run-up/run-up-dialog'
import { JobPanel } from '@/features/worktrees/running/components/job-panel'
import { JobSwitcher } from '@/features/worktrees/running/components/job-switcher'
import { RunGroup } from '@/features/worktrees/running/components/run-group'
import { groupsOf, type RunningGroup } from '@/features/worktrees/running/utils/groups'
import type { RunningSearch } from '@/features/worktrees/running/utils/search'
import { plural } from '@/lib/plural'
import { repoName } from '@/lib/repo'
import { cn } from '@/lib/utils'

// shortcut: the jobs come from list's read, counted here; the jobs read of its own (with its counts, its order and the
// shared services' group) replaces it
export function RunningPage({ repo, search }: { repo: string; search: RunningSearch }) {
  const list = useQuery(readQuery(repo, 'list'))
  const { progress, running } = useRun(repo, 'run-up')
  const { plan } = useRunUpDialog(repo).ran
  const root = useQuery(reposQuery).data?.find((known) => known.id === repo)?.root
  const now = useQuery({ queryKey: ['now'], queryFn: () => Date.now(), refetchInterval: clockEvery }).data ?? 0

  // past its end, a start's progress holds until the first read after it lands: its groups would vanish in between
  const ended = progress?.endedAt
  const overlay = running || (ended !== undefined && list.dataUpdatedAt < ended) ? progress : undefined
  const groups = list.data ? groupsOf({ read: list.data, progress: overlay, plan }) : []
  // the job asked for, else the first the list shows
  const picked =
    pick(groups, search.worktree, search.job) ??
    groups.flatMap((group) => pick([group], group.worktree, undefined) ?? [])[0]
  const up = groups.reduce((count, group) => count + group.counts.running, 0)
  const crashed = groups.reduce((count, group) => count + group.counts.crashed, 0)

  const description = () => {
    if (!list.data) return 'Reading the jobs…'
    if (groups.length === 0) return root ? `Nothing running in ${repoName(root)}` : 'Nothing running'
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
          <StatePage icon={<Activity />} title="Nothing is running">
            Start a profile to run a worktree's jobs. They keep running when this page closes, and show here, live, from
            any worktree.
          </StatePage>
        </Centered>
      )
    return (
      <div className="@container/page flex h-full min-h-0">
        <div
          className={cn(
            'flex flex-col divide-y overflow-y-auto px-8 py-2',
            picked ? 'w-[400px] shrink-0 border-r @max-[1100px]/page:hidden' : 'flex-1',
          )}
        >
          {groups.map((group) => (
            <RunGroup
              key={group.worktree}
              repo={repo}
              group={group}
              now={now}
              selected={group.worktree === picked?.group.worktree ? picked.job.name : undefined}
            />
          ))}
        </div>
        {picked && (
          <JobPanel
            key={`${picked.group.worktree}/${picked.job.name}/${picked.job.started_at ?? ''}`}
            repo={repo}
            worktree={picked.group.worktree}
            profile={picked.group.profile}
            job={picked.job}
            now={now}
            switcher={
              <JobSwitcher
                repo={repo}
                groups={groups}
                worktree={picked.group.worktree}
                job={picked.job.name}
                className="@min-[1100px]/page:hidden"
              />
            }
          />
        )}
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
            <ReasonButton
              disabled={groups.length === 0}
              why={groups.length === 0 ? 'Nothing is running' : undefined}
              onClick={() => openRunDown(repo)}
              variant="outline"
            >
              Stop jobs
            </ReasonButton>
            {start}
          </div>
        }
      />
      <RunUpDialog repo={repo} />
      <RunDownDialog repo={repo} />
      <div className="min-h-0 flex-1 overflow-hidden" aria-busy={!list.data}>
        {body()}
      </div>
    </>
  )
}

// the job of groups in worktree called job (the first of the worktree when job is none)
function pick(groups: RunningGroup[], worktree: string | undefined, job: string | undefined) {
  const group = groups.find((one) => one.worktree === worktree)
  const row = group?.rows.find((one) => 'job' in one && (job === undefined || one.name === job))
  return group && row && 'job' in row ? { group, job: row.job } : undefined
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
