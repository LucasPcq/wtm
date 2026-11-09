import { EyeOff } from 'lucide-react'
import type { RunpsJob } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { endedLine, endedShownLine } from '@/features/worktrees/running/utils/list-words'

// the row closing a group on its ended jobs: what they are, and Show / Hide
export function QuietRow({
  of,
  ended,
  shown,
  onToggle,
}: {
  of: string
  ended: RunpsJob[]
  shown: boolean
  onToggle: () => void
}) {
  return (
    <li className="flex items-center gap-2 py-1 text-xs text-muted-foreground">
      <EyeOff className="size-4 shrink-0" aria-hidden="true" />
      <span className="flex-1 truncate">{shown ? endedShownLine(ended) : endedLine(ended)}</span>
      <Button
        variant="ghost"
        size="sm"
        aria-label={`${shown ? 'Hide' : 'Show'} the ended jobs of ${of}`}
        aria-expanded={shown}
        onClick={onToggle}
      >
        {shown ? 'Hide' : 'Show'}
      </Button>
    </li>
  )
}
