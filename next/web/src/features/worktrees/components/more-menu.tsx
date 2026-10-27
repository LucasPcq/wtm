import { Ellipsis } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { ActionItems } from '@/features/worktrees/components/action-items'
import type { Subject } from '@/features/worktrees/utils/actions'
import { cn } from '@/lib/utils'

export function MoreMenu({
  repo,
  subject,
  rows,
  open,
  onOpenChange,
  disabled = false,
  className,
}: {
  repo: string
  subject: string
  rows: Subject[]
  open?: boolean
  onOpenChange?: (open: boolean) => void
  disabled?: boolean
  className?: string
}) {
  const [own, setOwn] = useState(false)
  // a worktree that goes under way closes its menu for good: it would open again once the run ends
  if (disabled && own) setOwn(false)

  return (
    <DropdownMenu open={(open ?? own) && !disabled} onOpenChange={onOpenChange ?? setOwn}>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={`Actions on ${subject}`}
          disabled={disabled}
          className={cn('relative z-10', className)}
        >
          <Ellipsis />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        <ActionItems kind="dropdown" repo={repo} rows={rows} />
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
