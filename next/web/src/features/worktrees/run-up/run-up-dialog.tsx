import { toast } from 'sonner'
import type { Applied } from '@/api/queries/commands'
import { Dialog } from '@/components/ui/dialog'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { succeeded, useRun } from '@/features/worktrees/hooks/use-run'
import { type RunUpRan, useRunUpDialog } from '@/features/worktrees/hooks/use-run-up-dialog'
import { RunUpProgress } from '@/features/worktrees/run-up/components/run-up-progress'
import { RunUpWizard } from '@/features/worktrees/run-up/components/run-up-wizard'
import { startedWords, where } from '@/features/worktrees/run-up/utils/words'
import { doneOf } from '@/features/worktrees/utils/progress'

const toastId = 'run-up'

const show = () => {
  toast.dismiss(toastId)
  useRunUpDialog.setState({ open: true })
}

// one start at a time: opened while one runs, or one that did not start is still to be read, it shows it
export function RunUpDialog({ repo }: { repo: string }) {
  const ask = useRunUpDialog()
  const run = useRun(repo, 'run-up')

  // hidden while it starts, the run goes on in a toast that follows it and opens it again
  const close = () => {
    useRunUpDialog.setState({ open: false })
    if (!run.running) return
    toast.loading(`Starting ${ask.ran.profile} ${where(ask.ran.worktrees)}`, {
      id: toastId,
      description: <HiddenProgress repo={repo} />,
      action: { label: 'Show', onClick: show },
      duration: Number.POSITIVE_INFINITY,
    })
  }

  const told = (ran: RunUpRan, envelope: Applied<'run-up'> | undefined, refused: string | undefined) => {
    // the live toast goes: one that ends replaces it, with none of its Show
    toast.dismiss(toastId)
    if (useRunUpDialog.getState().open) return
    const items = envelope?.items ?? []
    if (succeeded(envelope)) {
      const { title, description } = startedWords(ran.profile, envelope)
      return void toast.success(title, { description })
    }
    const failed = items.filter((item) => item.status !== 'done').length
    const said =
      items.length > 0
        ? `${ran.profile} didn't start in ${failed} of ${items.length}.`
        : (envelope?.error?.message ?? refused ?? 'Nothing started.')
    toast.error(said, { action: { label: 'Details', onClick: show } })
  }

  const content = () => {
    if (run.progress)
      return (
        <RunUpProgress
          progress={run.progress}
          running={run.running}
          ran={ask.ran}
          onHide={close}
          onCancel={run.cancel}
          onDone={() => {
            run.dismiss()
            useRunUpDialog.setState({ open: false })
          }}
        />
      )

    if (!ask.open) return null
    return (
      <RunUpWizard
        key={[ask.step, ask.profile, ...ask.worktrees].join('\n')}
        repo={repo}
        ask={ask}
        onStart={(start, ran) => {
          useRunUpDialog.setState({ ran })
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

function HiddenProgress({ repo }: { repo: string }) {
  const { progress } = useRun(repo, 'run-up')
  if (!progress) return null
  const { done, total } = doneOf(progress)
  return `${done} of ${total} up`
}
