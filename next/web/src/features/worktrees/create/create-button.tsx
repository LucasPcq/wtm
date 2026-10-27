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
import { ProgressView } from '@/features/worktrees/create/components/progress-view'
import { Wizard } from '@/features/worktrees/create/components/wizard'
import { succeeded, useRun } from '@/features/worktrees/hooks/use-run'

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
  const run = useRun(repo, 'create')
  const form = useQuery({ ...questionsQuery(repo, 'create'), enabled: open })
  const navigate = useNavigate({ from: '/r/$repo/worktrees' })

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

    if (form.data)
      return (
        <Wizard
          key={round}
          repo={repo}
          form={form.data}
          onStart={(start) =>
            run.start(start, (envelope) => {
              if (shown.current) return
              const created = start.units.filter((unit) => unit.phases.some((run) => run.phase !== 'fast_forward'))
              const names = created.map((unit) => unit.subject).join(', ')
              if (succeeded(envelope))
                return void toast.success(`${names} ${created.length === 1 ? 'is' : 'are'} ready.`)
              toast.error(`Couldn't create ${names}`, { action: { label: 'Details', onClick: () => setOpen(true) } })
            })
          }
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
