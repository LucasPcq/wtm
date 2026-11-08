import { useQuery } from '@tanstack/react-query'
import { Activity, Play } from 'lucide-react'
import { useState } from 'react'
import { hereIn } from '@/api/here'
import { jobsQuery } from '@/api/queries/commands'
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
import { RunStartDialog } from '@/features/worktrees/run-start/run-start-dialog'
import { RunStopDialog } from '@/features/worktrees/run-stop/run-stop-dialog'
import { RunUpDialog } from '@/features/worktrees/run-up/run-up-dialog'
import { JobPanel } from '@/features/worktrees/running/components/job-panel'
import { JobSwitcher } from '@/features/worktrees/running/components/job-switcher'
import { PinnedGroup } from '@/features/worktrees/running/components/pinned-group'
import { RunGroup } from '@/features/worktrees/running/components/run-group'
import { RunningToolbar } from '@/features/worktrees/running/components/running-toolbar'
import { groupJob, groupsOf, type RunningGroup } from '@/features/worktrees/running/utils/groups'
import type { RunningSearch } from '@/features/worktrees/running/utils/search'
import { pinOf, visible } from '@/features/worktrees/running/utils/visible'
import { useMarks, useRunFilter } from '@/features/worktrees/store/ui-store'
import { plural } from '@/lib/plural'
import { repoName } from '@/lib/repo'
import { cn } from '@/lib/utils'

export function RunningPage({ repo, search }: { repo: string; search: RunningSearch }) {
  const root = useQuery(reposQuery).data?.find((known) => known.id === repo)?.root
  const ps = useQuery(jobsQuery(repo, root))
  const { progress, running } = useRun(repo, 'run-up')
  const { plan } = useRunUpDialog(repo).ran
  const now = useQuery({ queryKey: ['now'], queryFn: () => Date.now(), refetchInterval: clockEvery }).data ?? 0
  const folded = useMarks('folded', repo)
  const pinned = useMarks('pinned', repo)
  const quiet = useMarks('quiet', repo)
  const { filter, change, clear } = useRunFilter(repo)
  const [typed, setTyped] = useState('')
  const here = hereIn(repo)

  // past its end, a start's progress holds until the first read after it lands: its groups would vanish in between
  const ended = progress?.endedAt
  const overlay = running || (ended !== undefined && ps.dataUpdatedAt < ended) ? progress : undefined
  const groups = ps.data ? groupsOf({ read: ps.data, progress: overlay, plan }) : []
  const shown = visible({ groups, filter, search: typed, here, pinned: pinned.has, quiet: quiet.has })
  // the shared services fold by default: a mark opens them, as one folds a worktree
  const isFolded = (group: RunningGroup) => group.shared !== folded.has(group.key)
  const pinnedKey = 'pinned'
  // the job asked for, shown or ended, else the first the page shows
  const picked =
    groupJob(groups, search.worktree, search.job) ??
    shown.pinned.map(({ group, row }) => ({ group, job: row.job }))[0] ??
    shown.groups.flatMap((group) => group.rows.flatMap((row) => ('job' in row ? [{ group, job: row.job }] : [])))[0]

  const worktrees = groups.map((group) => ({
    key: group.key,
    label: group.shared ? 'Shared services' : group.worktree,
  }))
  const up = groups.reduce((count, group) => count + group.counts.running, 0)
  const crashed = groups.reduce((count, group) => count + group.counts.crashed, 0)
  const live = groups.filter((group) => group.rows.length > 0)
  // the worktrees running something: each live group's, and each holder of a shared service
  const worktreesUp = new Set(
    live.flatMap((group) =>
      group.shared
        ? group.rows.flatMap((row) => ('job' in row ? (row.job.holders ?? []).map((holder) => holder.path) : []))
        : [group.path ?? group.worktree],
    ),
  ).size
  const hereName = groups.find((group) => !group.shared && group.path === here)?.worktree ?? here?.split('/').pop()

  const description = () => {
    if (!ps.data) return 'Reading the jobs…'
    if (live.length === 0) return root ? `Nothing running in ${repoName(root)}` : 'Nothing running'
    const where = `${plural(up, 'job')} up in ${plural(worktreesUp, 'worktree')}`
    return crashed > 0 ? `${where} · ${crashed} crashed` : where
  }

  const start = (
    <Button onClick={() => openRunUp(repo)}>
      <Play />
      Start a profile
    </Button>
  )

  const body = () => {
    if (ps.isError) return <ReadFailed message={ps.error.message} />
    if (!ps.data) return <Skeletons />
    if (live.length === 0 && !picked)
      return (
        <Centered>
          <StatePage icon={<Activity />} title="Nothing is running">
            Start a profile to run a worktree's jobs. They keep running when this page closes, and show here, live, from
            any worktree.
          </StatePage>
        </Centered>
      )
    return (
      <div className="@container/page flex h-full min-h-0 flex-col">
        <RunningToolbar
          search={typed}
          onSearch={setTyped}
          filter={filter}
          onFilter={change}
          onClear={() => {
            clear()
            setTyped('')
          }}
          counts={shown.counts}
          stateCounts={shown.states}
          hidden={shown.hidden}
          worktrees={worktrees}
          here={hereName}
          switcher={
            picked && (
              <JobSwitcher
                repo={repo}
                pinned={shown.pinned}
                groups={shown.groups}
                worktree={picked.group.worktree}
                job={picked.job.name}
                state={picked.job.state}
                className="@min-[1100px]/page:hidden"
              />
            )
          }
        />
        <div className="flex min-h-0 flex-1">
          <div
            className={cn(
              'flex flex-col divide-y overflow-y-auto px-8 py-2',
              picked ? 'w-[400px] shrink-0 border-r @max-[1100px]/page:hidden' : 'flex-1',
            )}
          >
            {shown.pinned.length > 0 && (
              <PinnedGroup
                repo={repo}
                pinned={shown.pinned}
                now={now}
                selected={picked && pinOf(picked.group.worktree, picked.job.name)}
                folded={folded.has(pinnedKey)}
                onFold={() => folded.toggle(pinnedKey)}
                onUnpinAll={pinned.clear}
              />
            )}
            {shown.groups.map((group) => (
              <RunGroup
                key={group.key}
                repo={repo}
                group={group}
                now={now}
                selected={group.key === picked?.group.key ? picked.job.name : undefined}
                folded={isFolded(group)}
                onFold={() => folded.toggle(group.key)}
                endedShown={group.endedShown}
                onQuiet={() => quiet.toggle(group.key)}
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
            />
          )}
        </div>
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
              disabled={live.length === 0}
              why={live.length === 0 ? 'Nothing is running' : undefined}
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
      <RunStartDialog repo={repo} />
      <RunStopDialog repo={repo} />
      <div className="min-h-0 flex-1 overflow-hidden" aria-busy={!ps.data}>
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
