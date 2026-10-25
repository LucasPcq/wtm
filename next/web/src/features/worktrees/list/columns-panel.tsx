import { Columns3, Lock } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { type ColumnId, columns, groups } from '@/features/worktrees/list/columns'
import { ChoicePopover } from '@/features/worktrees/shared/choice-popover'

export function ColumnsPanel({
  shown,
  onToggle,
  onReset,
}: {
  shown: ColumnId[]
  onToggle: (column: ColumnId) => void
  onReset: () => void
}) {
  return (
    <ChoicePopover
      trigger={
        <Button variant="outline" className="data-[state=open]:bg-accent">
          <Columns3 />
          Columns
        </Button>
      }
      head={
        <>
          <div className="flex items-center gap-2 border-b px-4 py-3">
            <span className="text-sm font-semibold">Columns</span>
            <span className="text-xs text-muted-foreground">Saved for this repository</span>
            <Button variant="ghost" size="sm" className="ml-auto" onClick={onReset}>
              Reset
            </Button>
          </div>
          <p className="flex items-center gap-1.5 px-4 pt-2.5 text-xs text-muted-foreground">
            <Lock className="size-3" aria-hidden="true" />
            Branch is always shown
          </p>
        </>
      }
      groups={groups.map((title) => ({
        title,
        choices: columns
          .filter((column) => column.meta?.group === title)
          .map((column) => ({ id: column.id, label: String(column.header), hint: column.meta?.hint })),
      }))}
      shown={shown}
      onToggle={(id) => {
        const column = columns.find((known) => known.id === id)
        if (column) onToggle(column.id)
      }}
      className="w-[540px]"
    />
  )
}
