import { TriangleAlert } from 'lucide-react'
import type { FastforwardAdvance } from '@/api/types.gen'
import { Checkbox } from '@/components/ui/checkbox'
import { Item, ItemTitle } from '@/components/ui/item'
import { unsafe } from '@/features/worktrees/fast-forward/utils/plan'
import { blockerWords, standingWords } from '@/features/worktrees/fast-forward/utils/words'

// a fast-forward loses nothing it would overwrite (git refuses then): checked anyway, the reason stays amber
export function AdvanceItem({
  advance,
  checked = true,
  onCheck,
}: {
  advance: FastforwardAdvance
  checked?: boolean
  onCheck?: (on: boolean) => void
}) {
  return (
    <Item role="listitem" aria-label={advance.worktree} className="items-center gap-3 rounded-none px-4 py-3">
      {onCheck && (
        <Checkbox
          aria-label={`Fast-forward ${advance.worktree}`}
          checked={checked}
          onCheckedChange={(on) => onCheck(on === true)}
        />
      )}
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <ItemTitle className="min-w-0">{advance.worktree}</ItemTitle>
        <div className="flex flex-col gap-1 text-xs">
          {(advance.blockers ?? []).map((blocker) => (
            <span key={blocker} className="flex items-center gap-1.5">
              <TriangleAlert className="size-3.5 shrink-0 text-amber-500" aria-hidden="true" />
              {blockerWords[blocker]}
            </span>
          ))}
          <span className="text-muted-foreground">{standingWords[advance.standing](advance)}</span>
        </div>
      </div>
      {unsafe(advance) && checked && <span className="shrink-0 text-xs text-muted-foreground">anyway</span>}
    </Item>
  )
}
