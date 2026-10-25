import { CircleCheck, CircleX, LoaderCircle, Minus, Square, TriangleAlert } from 'lucide-react'
import type { DomainItem } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { phaseLabel } from '@/features/worktrees/create/utils/words'
import { type Progress, settled, type Unit } from '@/features/worktrees/remove/utils/progress'

const statusLook: Record<DomainItem['status'], { Icon: typeof CircleCheck; className: string; label: string }> = {
  done: { Icon: CircleCheck, className: 'text-green-600 dark:text-green-500', label: 'Removed' },
  failed: { Icon: CircleX, className: 'text-destructive', label: 'Failed' },
  skipped: { Icon: Minus, className: 'text-muted-foreground', label: 'Not reached' },
  cancelled: { Icon: Minus, className: 'text-muted-foreground', label: 'Cancelled' },
}

export function RemoveProgress({
  progress,
  running,
  onHide,
  onCancel,
  onClose,
}: {
  progress: Progress
  running: boolean
  onHide: () => void
  onCancel: (force: boolean) => void
  onClose: () => void
}) {
  const units = settled(progress)
  const envelope = progress.envelope
  const removed = units.filter((unit) => unit.status === 'done').length
  const warnings = envelope?.warnings ?? []
  const failures = envelope?.items.filter((item) => item.error) ?? []

  const title = () => {
    if (running)
      return progress.cancelling
        ? 'Cancelling'
        : `Removing ${units.length === 1 ? units[0]?.subject : `${units.length} worktrees`}`
    if (progress.refused) return "Couldn't remove"
    return `${removed} ${removed === 1 ? 'worktree' : 'worktrees'} removed`
  }

  const state = (unit: Unit) => {
    if (unit.status) {
      const { Icon, className, label } = statusLook[unit.status]
      return (
        <span className="flex items-center gap-1.5 text-sm">
          <Icon className={`size-3.5 ${className}`} />
          {label}
        </span>
      )
    }
    if (!unit.phase) return <span className="text-sm text-muted-foreground">Waiting</span>
    return (
      <span className="flex min-w-0 items-center gap-1.5 text-sm text-muted-foreground">
        <LoaderCircle className="size-3.5 shrink-0 animate-spin" />
        <span className="truncate">{unit.line ?? phaseLabel[unit.phase]}</span>
      </span>
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{title()}</DialogTitle>
        <DialogDescription>
          {running && !progress.cancelling
            ? 'Closing this keeps it running.'
            : 'Each went with its branch and its record.'}
        </DialogDescription>
      </DialogHeader>

      <ul aria-label="Worktrees" className="flex flex-col">
        {units.map((unit) => (
          <li key={unit.subject} className="flex items-center gap-3 border-b py-2.5">
            <span className="min-w-0 flex-1 truncate text-sm font-medium">{unit.subject}</span>
            {state(unit)}
          </li>
        ))}
      </ul>

      {progress.refused && <p className="text-sm text-destructive">{progress.refused}</p>}
      {failures.map((item) => (
        <p key={item.subject} className="text-sm text-destructive">
          {item.subject}: {item.error?.message}
        </p>
      ))}
      {warnings.map((warning) => (
        <div key={`${warning.code}-${warning.message}`} className="flex flex-col gap-2">
          <p className="flex items-center gap-2 text-sm">
            <TriangleAlert className="size-3.5 shrink-0 text-amber-600 dark:text-amber-500" />
            {warning.message}
          </p>
          {warning.follow_up && <CommandLine followUp={warning.follow_up} />}
        </div>
      ))}

      <DialogFooter className="items-center">
        {running ? (
          <>
            <Button variant="ghost" className="mr-auto" onClick={onHide}>
              Hide
            </Button>
            <Button
              variant="outline"
              disabled={!progress.operation || progress.cancelling === 'forced'}
              onClick={() => onCancel(progress.cancelling === 'asked')}
            >
              <Square />
              {progress.cancelling ? 'Stop now' : 'Cancel'}
            </Button>
          </>
        ) : (
          <Button onClick={onClose}>Close</Button>
        )}
      </DialogFooter>
    </>
  )
}
