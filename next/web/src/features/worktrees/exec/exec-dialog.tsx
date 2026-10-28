import { toast } from 'sonner'
import type { Applied } from '@/api/queries/commands'
import { Dialog } from '@/components/ui/dialog'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { ExecProgress } from '@/features/worktrees/exec/components/exec-progress'
import { ExecWizard } from '@/features/worktrees/exec/components/exec-wizard'
import { lockHeld, passedWords } from '@/features/worktrees/exec/utils/words'
import { confirmExec, useExecDialog } from '@/features/worktrees/hooks/use-exec-dialog'
import { succeeded, useRun } from '@/features/worktrees/hooks/use-run'
import { subjectsOf } from '@/features/worktrees/utils/actions'

// one run at a time, as Remove: opened while one runs, or one that did not pass is still to be read, it shows it
export function ExecDialog({ repo }: { repo: string }) {
  const ask = useExecDialog()
  const run = useRun(repo, 'exec')
  const close = () => useExecDialog.setState({ open: false })

  const told = (envelope: Applied<'exec'> | undefined, asked: number, refused: string | undefined) => {
    if (useExecDialog.getState().open) return
    const { command } = useExecDialog.getState().ran
    const items = envelope?.items ?? []
    if (succeeded(envelope)) return void toast.success(passedWords(command, items))
    const failed = asked - items.filter((item) => item.status === 'done').length
    const said =
      items.length > 0
        ? `${command} didn't pass in ${failed} of ${asked}.`
        : (envelope?.error?.message ?? refused ?? 'Nothing ran.')
    toast.error(said, { action: { label: 'Details', onClick: () => useExecDialog.setState({ open: true }) } })
  }

  const content = () => {
    if (run.progress)
      return (
        <ExecProgress
          progress={run.progress}
          running={run.running}
          ran={ask.ran}
          onHide={close}
          onCancel={run.cancel}
          onDone={() => {
            run.dismiss()
            close()
          }}
          onAgain={(retry) => {
            run.dismiss()
            const [command = ''] = retry.answers?.find((answer) => answer.path === 'command')?.values ?? []
            confirmExec({ worktrees: subjectsOf(retry), command })
          }}
        />
      )

    if (!ask.open) return null
    return (
      <ExecWizard
        key={[ask.step, ask.command, ask.held?.params?.op, ...ask.worktrees].join('\n')}
        repo={repo}
        ask={ask}
        onStart={(start, ran) => {
          useExecDialog.setState({ ran })
          run.start(start, (envelope, refused) => {
            // a lock is the tool's: the plan stands, so the dialog goes back to it and says who holds what. The
            // operation began before its locks were refused, so the refusal comes in its envelope, with no item
            const held = envelope?.items.length === 0 ? envelope.error : undefined
            if (held?.code === lockHeld) {
              run.dismiss()
              return confirmExec({ worktrees: ran.worktrees, command: ran.command, held })
            }
            told(envelope, start.units.length, refused)
          })
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
