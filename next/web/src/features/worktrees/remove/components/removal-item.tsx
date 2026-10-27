import { CircleX, ExternalLink, TriangleAlert } from 'lucide-react'
import type { TeardownMove, TeardownRemoval } from '@/api/types.gen'
import { Checkbox } from '@/components/ui/checkbox'
import { Item, ItemTitle } from '@/components/ui/item'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { unsafe } from '@/features/worktrees/remove/utils/plan'
import { blockerWords, lostWords } from '@/features/worktrees/remove/utils/words'

export function RemovalItem({
  removal,
  checked = true,
  onCheck,
  moves = [],
  reparent = false,
}: {
  removal: TeardownRemoval
  checked?: boolean
  onCheck?: (on: boolean) => void
  moves?: TeardownMove[]
  reparent?: boolean
}) {
  const blockers = removal.blockers ?? []
  const anyway = unsafe(removal) && checked

  const name = () => {
    if (!onCheck || !removal.path) return removal.worktree
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          {/* biome-ignore lint/a11y/noNoninteractiveTabindex: the path shows to a keyboard as to a pointer */}
          <span tabIndex={0}>{removal.worktree}</span>
        </TooltipTrigger>
        <TooltipContent className="max-w-none font-mono">{removal.path}</TooltipContent>
      </Tooltip>
    )
  }

  const exception = () => {
    if (!removal.path) return 'A branch with no worktree: deleted alone.'
    if (!removal.branch) return 'On no branch: only the worktree goes.'
    return undefined
  }

  const odd = exception()

  return (
    <Item role="listitem" aria-label={removal.worktree} className="items-center gap-3 rounded-none px-4 py-3">
      {onCheck && (
        <Checkbox
          aria-label={`Remove ${removal.worktree}`}
          checked={checked}
          onCheckedChange={(on) => onCheck(on === true)}
        />
      )}
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <ItemTitle className="min-w-0">{name()}</ItemTitle>
        <div className="flex flex-col gap-1 text-xs">
          {blockers.map((blocker) => (
            <span key={blocker} className="flex items-center gap-1.5">
              {anyway ? (
                <CircleX className="size-3.5 shrink-0 text-destructive" aria-hidden="true" />
              ) : (
                <TriangleAlert className="size-3.5 shrink-0 text-amber-500" aria-hidden="true" />
              )}
              {anyway ? lostWords[blocker](removal) : blockerWords[blocker](removal)}
              {!anyway && blocker === 'open_pull_request' && removal.pull_request && (
                <a href={removal.pull_request} target="_blank" rel="noreferrer" aria-label="Open the pull request">
                  <ExternalLink className="size-3.5" />
                </a>
              )}
            </span>
          ))}
          {moves.map((move) => (
            <span key={move.branch} className="text-muted-foreground">
              {reparent ? `${move.branch} moves onto ${move.to}` : `${move.branch} stays on ${move.from}, orphaned`}
            </span>
          ))}
          {odd && <span className="text-muted-foreground">{odd}</span>}
        </div>
      </div>
      {anyway && <span className="shrink-0 text-xs text-muted-foreground">anyway</span>}
    </Item>
  )
}
