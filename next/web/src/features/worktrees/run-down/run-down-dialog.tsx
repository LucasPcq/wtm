import { toast } from 'sonner'
import type { Applied } from '@/api/queries/commands'
import { RunDialogContent } from '@/components/run-dialog-content'
import { Dialog } from '@/components/ui/dialog'
import { StopProgress } from '@/features/worktrees/components/stop-progress'
import { succeeded, useRun } from '@/features/worktrees/hooks/use-run'
import {
  type RunDownRan,
  runDownDialogOf,
  setRunDownDialog,
  showRunDown,
  toastIdOf,
  useRunDownDialog,
} from '@/features/worktrees/hooks/use-run-down-dialog'
import { RunDownWizard } from '@/features/worktrees/run-down/components/run-down-wizard'
import { where } from '@/features/worktrees/utils/run-words'
import { countOf } from '@/features/worktrees/utils/stop-plan'

// one stop at a time: opened while one runs, or one that failed is still to be read, it shows it
export function RunDownDialog({ repo }: { repo: string }) {
  const ask = useRunDownDialog(repo)
  const run = useRun(repo, 'run-down')
  const toastId = toastIdOf(repo)

  // hidden while it stops, the run goes on in a toast that follows it and opens it again
  const close = () => {
    setRunDownDialog(repo, { open: false })
    if (!run.running) return
    toast.loading(`Stopping jobs ${where(ask.ran.worktrees)}`, {
      id: toastId,
      description: <HiddenProgress repo={repo} />,
      action: { label: 'Show', onClick: () => showRunDown(repo) },
      duration: Number.POSITIVE_INFINITY,
    })
  }

  const told = (envelope: Applied<'run-down'> | undefined, refused: string | undefined) => {
    toast.dismiss(toastId)
    if (runDownDialogOf(repo).open) return
    const items = envelope?.items ?? []
    if (succeeded(envelope)) {
      toast.success(`Jobs stopped ${where(items.map((item) => item.subject))}`)
      return run.dismiss()
    }
    const failed = items.filter((item) => item.status !== 'done').length
    const said = () => {
      if (failed > 0) return `Jobs didn't stop in ${failed} of ${items.length}.`
      return envelope?.error?.message ?? refused ?? 'Nothing stopped.'
    }
    toast.error(said(), {
      action: { label: 'Details', onClick: () => showRunDown(repo) },
    })
  }

  const content = () => {
    if (run.progress)
      return (
        <StopProgress
          title="Stop jobs"
          stopping={`Stopping jobs ${where(ask.ran.worktrees)}`}
          progress={run.progress}
          running={run.running}
          ran={ask.ran}
          onHide={close}
          onDone={() => {
            run.dismiss()
            setRunDownDialog(repo, { open: false })
          }}
        />
      )

    if (!ask.open) return null
    return (
      <RunDownWizard
        key={ask.worktrees.join('\n')}
        repo={repo}
        ask={ask}
        onStop={(start, ran: RunDownRan) => {
          setRunDownDialog(repo, { ran })
          run.start(start, told)
        }}
        onCancel={close}
      />
    )
  }

  return (
    <Dialog open={ask.open} onOpenChange={(next) => !next && close()}>
      <RunDialogContent running={run.running}>{content()}</RunDialogContent>
    </Dialog>
  )
}

function HiddenProgress({ repo }: { repo: string }) {
  const { progress } = useRun(repo, 'run-down')
  const { plan } = useRunDownDialog(repo).ran
  if (!progress) return null
  const { done, total } = countOf(progress.units, plan)
  return `${done} of ${total} stopped`
}
