import { useQuery } from '@tanstack/react-query'
import { Trash2 } from 'lucide-react'
import { questionsQuery } from '@/api/queries/commands'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { openRemove, useRemoveDialog } from '@/features/worktrees/hooks/use-remove-dialog'
import { RemoveProgress } from '@/features/worktrees/remove/components/remove-progress'
import { RemoveWizard } from '@/features/worktrees/remove/components/remove-wizard'
import { useRemoveRun } from '@/features/worktrees/remove/hooks/use-remove-run'

export function RemoveDialog({ repo }: { repo: string }) {
  const { open, preset } = useRemoveDialog()
  const run = useRemoveRun(repo)
  const form = useQuery({ ...questionsQuery(repo, 'clean'), enabled: open })
  const close = () => useRemoveDialog.setState({ open: false })

  const content = () => {
    if (run.progress)
      return (
        <RemoveProgress
          progress={run.progress}
          running={run.running}
          onHide={close}
          onCancel={run.cancel}
          onClose={() => {
            run.dismiss()
            close()
          }}
        />
      )

    if (form.data)
      return (
        <RemoveWizard
          key={preset.join('\n')}
          repo={repo}
          form={form.data}
          preset={preset}
          onStart={run.start}
          onCancel={close}
        />
      )

    return (
      <DialogHeader>
        <DialogTitle>Remove worktrees</DialogTitle>
        <DialogDescription>{form.isError ? form.error.message : 'Asking wtm what it can remove…'}</DialogDescription>
        {form.isPending && <Skeleton className="h-40 w-full" />}
      </DialogHeader>
    )
  }

  return (
    <Dialog open={open} onOpenChange={(next) => (next ? openRemove([]) : close())}>
      <Button variant="outline" onClick={() => openRemove([])}>
        <Trash2 />
        Remove…
      </Button>
      <DialogContent className="flex max-h-[90vh] flex-col gap-6 sm:max-w-xl" showCloseButton={!run.running}>
        {content()}
      </DialogContent>
    </Dialog>
  )
}
