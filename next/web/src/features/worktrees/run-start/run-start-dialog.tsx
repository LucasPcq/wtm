import { toast } from 'sonner'
import type { Applied } from '@/api/queries/commands'
import { Dialog } from '@/components/ui/dialog'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { useRun } from '@/features/worktrees/hooks/use-run'
import {
  type RunStartRan,
  runStartDialogOf,
  setRunStartDialog,
  showRunStart,
  toastIdOf,
  useRunStartDialog,
} from '@/features/worktrees/hooks/use-run-start-dialog'
import { RunStartProgress } from '@/features/worktrees/run-start/components/run-start-progress'
import { RunStartWizard } from '@/features/worktrees/run-start/components/run-start-wizard'
import { startedTold } from '@/features/worktrees/utils/job-told'

// one start at a time, the dialog's or a job's direct one: opened while one runs, or one that failed is still to be
// read, it shows it
export function RunStartDialog({ repo }: { repo: string }) {
  const ask = useRunStartDialog(repo)
  const run = useRun(repo, 'run-start')
  const toastId = toastIdOf(repo)

  const close = () => {
    setRunStartDialog(repo, { open: false })
    if (!run.running) return
    const { ran } = runStartDialogOf(repo)
    toast.loading(ran.restart ? `Restarting ${ran.job}` : `Starting ${ran.job}`, {
      id: toastId,
      description: ran.worktree,
      action: { label: 'Show', onClick: () => showRunStart(repo) },
      duration: Number.POSITIVE_INFINITY,
    })
  }

  const told = (ran: RunStartRan, envelope: Applied<'run-start'> | undefined, refused: string | undefined) => {
    toast.dismiss(toastId)
    if (runStartDialogOf(repo).open) return
    const { tone, title, description } = startedTold({ at: ran, restart: ran.restart }, envelope, refused)
    if (tone !== 'error') {
      toast[tone](title, { description })
      return run.dismiss()
    }
    toast.error(title, { description, action: { label: 'Details', onClick: () => showRunStart(repo) } })
  }

  const content = () => {
    if (run.progress)
      return (
        <RunStartProgress
          progress={run.progress}
          running={run.running}
          ran={ask.ran}
          onHide={close}
          onDone={() => {
            run.dismiss()
            setRunStartDialog(repo, { open: false })
          }}
        />
      )

    if (!ask.open) return null
    return (
      <RunStartWizard
        key={[ask.step, ask.worktree, ask.job, ask.restart].join('\n')}
        repo={repo}
        ask={ask}
        onStart={(start, ran) => {
          setRunStartDialog(repo, { ran })
          run.start(start, (envelope, refused) => told(ran, envelope, refused))
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
