import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { questionsQuery } from '@/api/queries/commands'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { ProgressView } from '@/features/worktrees/create/progress-view'
import { Wizard } from '@/features/worktrees/create/wizard'
import { useCreateRun } from '@/features/worktrees/hooks/use-create-run'
import { cn } from '@/lib/utils'

export function CreateButton({ repo }: { repo: string }) {
  const [open, setOpen] = useState(false)
  // a new round mounts a fresh wizard: Create another starts from nothing typed
  const [round, setRound] = useState(0)
  const run = useCreateRun(repo)
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
      return <Wizard key={round} repo={repo} form={form.data} onStart={run.start} onCancel={() => setOpen(false)} />

    return (
      <>
        <DialogHeader>
          <DialogTitle>Create worktree</DialogTitle>
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
      <DialogContent
        // a run keeps one size while its phases come and go, only its lists scroll; once it ended, the dialog eases
        // to what it has left to say
        className={cn(
          'flex max-h-[90vh] flex-col gap-6 transition-[height] duration-300 ease-out [interpolate-size:allow-keywords] sm:max-w-xl motion-reduce:transition-none',
          run.running && 'h-[min(36rem,85vh)]',
        )}
        showCloseButton={!run.running}
      >
        {content()}
      </DialogContent>
    </Dialog>
  )
}
