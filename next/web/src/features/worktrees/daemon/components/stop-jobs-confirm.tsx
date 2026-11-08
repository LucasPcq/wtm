import { useQuery } from '@tanstack/react-query'
import { Play } from 'lucide-react'
import { dryRunQuery, type Planned } from '@/api/queries/commands'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert } from '@/features/worktrees/components/alert'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { RepoRowsList } from '@/features/worktrees/daemon/components/repo-rows'
import { stopRows, stopTotal } from '@/features/worktrees/daemon/utils/rows'

export function StopJobsConfirm({
  repo,
  onStop,
  onCancel,
}: {
  repo: string
  onStop: (fingerprint: string, plan: Planned<'daemon-stop-jobs'>) => void
  onCancel: () => void
}) {
  const planned = useQuery(dryRunQuery(repo, 'daemon-stop-jobs', []))
  const plan = planned.data?.error ? undefined : planned.data?.plan
  const fingerprint = planned.data?.fingerprint
  const refusal = planned.data?.error ?? undefined
  const failed = planned.isError ? planned.error.message : refusal?.message
  const total = plan ? stopTotal(plan) : 0

  const label = () => {
    if (total === 0) return 'Nothing to stop'
    return total === 1 ? 'Stop 1 job' : `Stop ${total} jobs`
  }

  const body = () => {
    if (failed)
      return (
        <Alert
          tone="red"
          description={failed}
          below={refusal?.follow_up && <CommandLine followUp={refusal.follow_up} />}
        />
      )
    if (!plan) return <Skeleton className="h-40 w-full" />
    if (total === 0) return <p className="text-sm text-muted-foreground">No repository runs a job.</p>
    return (
      <>
        <RepoRowsList repos={stopRows(plan)} />
        <section className="flex flex-col gap-2">
          <h3 className="text-sm text-muted-foreground">How it runs</h3>
          <p className="flex items-center gap-2 text-sm">
            <Play className="size-4 text-muted-foreground" aria-hidden="true" />
            Nothing starts again on its own.
          </p>
        </section>
      </>
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Stop every job</DialogTitle>
        <DialogDescription>
          In every repository. The proxy stays up: named addresses answer again once a job starts.
        </DialogDescription>
      </DialogHeader>
      <DialogScroll>{body()}</DialogScroll>
      <DialogFooter className="items-center">
        <Button variant="ghost" className="mr-auto" onClick={onCancel}>
          Cancel
        </Button>
        <Button
          variant="destructive"
          disabled={!plan || !fingerprint || total === 0 || planned.isFetching}
          onClick={() => plan && fingerprint && onStop(fingerprint, plan)}
        >
          {label()}
        </Button>
      </DialogFooter>
    </>
  )
}
