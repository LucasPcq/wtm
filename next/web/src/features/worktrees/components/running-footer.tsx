import { Square } from 'lucide-react'
import type { MutationCommand } from '@/api/queries/commands'
import { Button } from '@/components/ui/button'
import type { Progress } from '@/features/worktrees/utils/progress'

export function RunningFooter<C extends MutationCommand>({
  progress,
  stopping,
  onHide,
  onCancel,
}: {
  progress: Progress<C>
  stopping: string
  onHide: () => void
  onCancel: (force: boolean) => void
}) {
  return (
    <>
      <p className="mr-auto text-sm text-muted-foreground">
        {progress.cancelling ? stopping : 'Closing this keeps it running.'}
      </p>
      <Button variant="ghost" onClick={onHide}>
        Hide
      </Button>
      {progress.cancelling ? (
        <Button variant="destructive" disabled={progress.cancelling === 'forced'} onClick={() => onCancel(true)}>
          Stop now
        </Button>
      ) : (
        <Button variant="outline" disabled={!progress.operation} onClick={() => onCancel(false)}>
          <Square />
          Cancel
        </Button>
      )}
    </>
  )
}
