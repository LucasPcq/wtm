import { type QueryClient, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { type Applied, readQuery } from '@/api/queries/commands'
import { Dialog } from '@/components/ui/dialog'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { succeeded, useRun } from '@/features/worktrees/hooks/use-run'
import {
  type RunUpRan,
  runUpDialogOf,
  setRunUpDialog,
  showRunUp,
  toastIdOf,
  useRunUpDialog,
} from '@/features/worktrees/hooks/use-run-up-dialog'
import { RunUpProgress } from '@/features/worktrees/run-up/components/run-up-progress'
import { RunUpWizard } from '@/features/worktrees/run-up/components/run-up-wizard'
import { countOf } from '@/features/worktrees/run-up/utils/plan'
import { startedWords, where } from '@/features/worktrees/run-up/utils/words'

// resolves once the list read lands after at: never while no list is read, the run then kept until the dialog opens
const listedAfter = (client: QueryClient, repo: string, at: number) =>
  new Promise<void>((resolve) => {
    const { queryKey } = readQuery(repo, 'list')
    const landed = () => (client.getQueryState(queryKey)?.dataUpdatedAt ?? 0) > at
    if (landed()) return resolve()
    const stop = client.getQueryCache().subscribe(() => {
      if (!landed()) return
      stop()
      resolve()
    })
  })

// one start at a time: opened while one runs, or one that did not start is still to be read, it shows it
export function RunUpDialog({ repo }: { repo: string }) {
  const client = useQueryClient()
  const ask = useRunUpDialog(repo)
  const run = useRun(repo, 'run-up')
  const toastId = toastIdOf(repo)

  // hidden while it starts, the run goes on in a toast that follows it and opens it again
  const close = () => {
    setRunUpDialog(repo, { open: false })
    if (!run.running) return
    toast.loading(`Starting ${ask.ran.profile} ${where(ask.ran.worktrees)}`, {
      id: toastId,
      description: <HiddenProgress repo={repo} />,
      action: { label: 'Show', onClick: () => showRunUp(repo) },
      duration: Number.POSITIVE_INFINITY,
    })
  }

  const told = (ran: RunUpRan, envelope: Applied<'run-up'> | undefined, refused: string | undefined) => {
    // the live toast goes: one that ends replaces it, with none of its Show
    toast.dismiss(toastId)
    if (runUpDialogOf(repo).open) return
    const items = envelope?.items ?? []
    if (succeeded(envelope)) {
      const { title, description } = startedWords(ran.profile, envelope)
      toast.success(title, { description })
      // read where it ended: the page keeps its rows until the list lands, then the run has nothing left to show
      return void listedAfter(client, repo, Date.now()).then(run.dismiss)
    }
    const failed = items.filter((item) => item.status !== 'done').length
    const said =
      items.length > 0
        ? `${ran.profile} didn't start in ${failed} of ${items.length}.`
        : (envelope?.error?.message ?? refused ?? 'Nothing started.')
    toast.error(said, { action: { label: 'Details', onClick: () => showRunUp(repo) } })
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
            setRunUpDialog(repo, { open: false })
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
          setRunUpDialog(repo, { ran })
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
  const { plan } = useRunUpDialog(repo).ran
  if (!progress) return null
  const { done, total } = countOf(progress.units, plan)
  return `${done} of ${total} up`
}
