import { useQuery } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { dryRunQuery } from '@/api/queries/commands'
import type { DaemonrestartPlan } from '@/api/types.gen'
import { DialogBand } from '@/components/dialog-band'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert } from '@/features/worktrees/components/alert'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { RepoRowsList } from '@/features/worktrees/daemon/components/repo-rows'
import { RowBox } from '@/features/worktrees/daemon/components/row-box'
import { notBackRows, restartRows, takenOverRows } from '@/features/worktrees/daemon/utils/rows'

export function RestartDialog({
  repo,
  onRestart,
  onCancel,
}: {
  repo: string
  onRestart: (fingerprint: string, plan: DaemonrestartPlan) => void
  onCancel: () => void
}) {
  const planned = useQuery(dryRunQuery(repo, 'daemon-restart', []))
  const plan = planned.data?.error ? undefined : planned.data?.plan
  const fingerprint = planned.data?.fingerprint
  const refusal = planned.data?.error ?? undefined
  const failed = planned.isError ? planned.error.message : refusal?.message

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

    const taken = takenOverRows(plan)
    const notBack = notBackRows(plan)
    return (
      <>
        <RepoRowsList repos={restartRows(plan)} />
        {taken.length > 0 && (
          <section className="flex flex-col gap-2">
            <h3 className="text-sm text-muted-foreground">Taken over, not restarted</h3>
            <RowBox label="Taken over" rows={taken} />
          </section>
        )}
        {notBack.length > 0 && (
          <section className="flex flex-col gap-2">
            <h3 className="text-sm text-muted-foreground">Not back</h3>
            <RowBox label="Not back" rows={notBack} />
          </section>
        )}
        <section className="flex flex-col gap-2">
          <h3 className="text-sm text-muted-foreground">How it runs</h3>
          <p className="flex items-center gap-2 text-sm">
            <RefreshCw className="size-4 text-muted-foreground" aria-hidden="true" />
            The proxy restarts too; this page reconnects on its own.
          </p>
        </section>
      </>
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Restart wtm's daemon</DialogTitle>
        <DialogDescription>
          {plan?.running === false
            ? 'It does not run: it starts, and brings back what a daemon that died left.'
            : "Every repository's jobs restart: a short cut, then they run again on the new daemon."}
        </DialogDescription>
      </DialogHeader>
      <DialogBand />
      <DialogScroll>{body()}</DialogScroll>
      <DialogFooter className="items-center">
        <Button variant="ghost" className="mr-auto" onClick={onCancel}>
          Cancel
        </Button>
        <Button
          disabled={!plan || !fingerprint || planned.isFetching}
          onClick={() => fingerprint && plan && onRestart(fingerprint, plan)}
        >
          Restart
        </Button>
      </DialogFooter>
    </>
  )
}
