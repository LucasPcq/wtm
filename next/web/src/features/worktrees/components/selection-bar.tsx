import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useRunEntry } from '@/features/worktrees/hooks/use-run-entry'
import { type MenuAction, menu, type Subject, selectionEntryOf } from '@/features/worktrees/utils/actions'
import { cn } from '@/lib/utils'

const actions: MenuAction[] = ['fast_forward', 'exec', 'reparent']

export function SelectionBar({
  repo,
  count,
  rows,
  onClear,
}: {
  repo: string
  count: number
  rows: Subject[]
  onClear: () => void
}) {
  const run = useRunEntry(repo)

  const button = (action: MenuAction) => {
    const entry = selectionEntryOf(rows, action)
    const { label, icon: Icon } = menu[action]
    const destructive = action === 'clean'
    const why = 'disabled' in entry ? entry.disabled : undefined

    return (
      <Tooltip key={action}>
        <TooltipTrigger asChild>
          {/* the wrapper takes the pointer a disabled button refuses, so its reason shows */}
          <span tabIndex={why ? 0 : -1}>
            <Button
              variant="outline"
              size="sm"
              aria-label={label}
              disabled={why !== undefined}
              onClick={() => run(entry, rows)}
              className={cn(destructive && 'border-destructive text-destructive hover:text-destructive')}
            >
              <Icon />
              <span className="max-xl:hidden">{label}</span>
            </Button>
          </span>
        </TooltipTrigger>
        <TooltipContent className={cn(!why && 'xl:hidden')}>{why ?? label}</TooltipContent>
      </Tooltip>
    )
  }

  return (
    <div role="toolbar" aria-label="Selection" className="flex h-[60px] shrink-0 items-center gap-2 border-b px-8">
      <span className="text-sm font-medium">{count} selected</span>
      <Button variant="ghost" size="sm" onClick={onClear}>
        Clear
      </Button>
      <div className="flex-1" />
      {actions.map(button)}
      <Separator orientation="vertical" className="mx-1 data-vertical:h-6 data-vertical:self-center" />
      {button('clean')}
    </div>
  )
}
