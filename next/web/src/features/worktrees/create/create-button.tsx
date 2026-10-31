import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { useRef, useState } from 'react'
import { toast } from 'sonner'
import { questionsQuery } from '@/api/queries/commands'
import { Button } from '@/components/ui/button'
import { Dialog, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { CheckoutPanel } from '@/features/worktrees/create/components/checkout-panel'
import { ProgressView } from '@/features/worktrees/create/components/progress-view'
import { StartFrom, type StartFromValue } from '@/features/worktrees/create/components/start-from'
import { Wizard } from '@/features/worktrees/create/components/wizard'
import { type Start, succeeded, useRun } from '@/features/worktrees/hooks/use-run'

export function CreateButton({ repo }: { repo: string }) {
  const [open, setShown] = useState(false)
  // read when a run ends, after the render that started it: a ref, not the state of that render
  const shown = useRef(false)
  const setOpen = (on: boolean) => {
    shown.current = on
    setShown(on)
  }
  // a new round mounts a fresh wizard: Create another starts from nothing typed
  const [round, setRound] = useState(0)
  const [from, setFrom] = useState<StartFromValue>('branch')
  const created = useRun(repo, 'create')
  // a pull request checked out is a worktree created: the same dialog, its run checkout's own
  const checkedOut = useRun(repo, 'checkout')
  const run = checkedOut.progress ? checkedOut : created
  const form = useQuery({ ...questionsQuery(repo, 'create'), enabled: open && from === 'branch' })
  const navigate = useNavigate({ from: '/r/$repo/worktrees' })

  const startFrom = <StartFrom value={from} onChange={setFrom} />

  const start = (runner: typeof created | typeof checkedOut) => (params: Start) =>
    runner.start(params, (envelope) => {
      if (shown.current) return
      const opened = params.units.filter((unit) => unit.phases.some((phase) => phase.phase !== 'fast_forward'))
      const names = opened.map((unit) => unit.subject).join(', ')
      if (succeeded(envelope)) return void toast.success(`${names} ${opened.length === 1 ? 'is' : 'are'} ready.`)
      toast.error(`Couldn't create ${names}`, { action: { label: 'Details', onClick: () => setOpen(true) } })
    })

  const content = () => {
    if (run.progress)
      return (
        <ProgressView
          progress={run.progress}
          running={run.running}
          onHide={() => setOpen(false)}
          onCancel={run.cancel}
          onAnother={() => {
            run.dismiss()
            setRound((n) => n + 1)
          }}
          onDone={(branch) => {
            run.dismiss()
            setOpen(false)
            if (branch) void navigate({ search: (search) => ({ ...search, worktree: branch }) })
          }}
        />
      )

    if (from === 'pull_request')
      return (
        <CheckoutPanel
          key={round}
          repo={repo}
          startFrom={startFrom}
          onStart={start(checkedOut)}
          onCancel={() => setOpen(false)}
          onNewBranch={() => setFrom('branch')}
        />
      )

    if (form.data)
      return (
        <Wizard
          key={round}
          repo={repo}
          form={form.data}
          startFrom={startFrom}
          onStart={start(created)}
          onCancel={() => setOpen(false)}
        />
      )

    return (
      <>
        <DialogHeader>
          <DialogTitle>Create worktrees</DialogTitle>
          <DialogDescription>{form.isError ? form.error.message : 'Asking wtm what it needs…'}</DialogDescription>
        </DialogHeader>
        {form.isPending && <Skeleton className="h-48 w-full" />}
      </>
    )
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Button onClick={() => setOpen(true)}>
        <Plus />
        Create worktree
      </Button>
      <RunDialogContent running={run.running}>{content()}</RunDialogContent>
    </Dialog>
  )
}
