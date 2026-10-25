import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { questionsQuery } from '@/api/queries/commands'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { useCreateRun } from '../hooks/use-create-run'
import { ProgressView } from './progress-view'
import { Wizard } from './wizard'

// CreateButton is the screen's one primary action and the dialog it opens: create's questions a step at a time,
// then its run; a run under way shows again when the dialog opens again.
export function CreateButton({ repo }: { repo: string }) {
  const [open, setOpen] = useState(false)
  const [round, setRound] = useState(0)
  const run = useCreateRun(repo)
  const form = useQuery({ ...questionsQuery(repo, 'create'), enabled: open })
  const navigate = useNavigate({ from: '/r/$repo/worktrees' })

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Button onClick={() => setOpen(true)}>
        <Plus />
        Create worktree
      </Button>
      <DialogContent
        // a run keeps one size while its phases come and go; only its lists scroll
        className={cn('flex max-h-[90vh] flex-col gap-6 sm:max-w-xl', run.progress && 'h-[min(36rem,85vh)]')}
        showCloseButton={!run.running}
      >
        {run.progress ? (
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
        ) : form.data ? (
          <Wizard key={round} repo={repo} form={form.data} onStart={run.start} onCancel={() => setOpen(false)} />
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>Create worktree</DialogTitle>
              <DialogDescription>{form.isError ? form.error.message : 'Asking wtm what it needs…'}</DialogDescription>
            </DialogHeader>
            {form.isPending && <Skeleton className="h-48 w-full" />}
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
