import { CircleX, ExternalLink, TriangleAlert } from 'lucide-react'
import type { ReactNode } from 'react'
import type { CleanMove, CleanRemoval } from '@/api/types.gen'
import { Item, ItemTitle } from '@/components/ui/item'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { type Choice, unsafe } from '@/features/worktrees/remove/utils/plan'
import { blockerWords, choiceWords, lostWords } from '@/features/worktrees/remove/utils/words'
import { cn } from '@/lib/utils'

// one worktree of a removal: on the first step, why it can't go safely and the choice to make; on Confirm, what it
// loses and what becomes of its children
export function RemovalItem({
  removal,
  choice,
  onChoice,
  moves = [],
  reparent = false,
}: {
  removal: CleanRemoval
  choice?: Choice | undefined
  onChoice?: (choice: Choice) => void
  moves?: CleanMove[]
  reparent?: boolean
}) {
  const blockers = removal.blockers ?? []
  const risky = unsafe(removal)
  const kept = choice === 'keep'

  const glyph = () => {
    if (!risky) return <span className="size-4 shrink-0" />
    if (!onChoice) return <CircleX className="size-4 shrink-0 text-destructive" aria-hidden="true" />
    return (
      <TriangleAlert
        className={cn('size-4 shrink-0', kept ? 'text-muted-foreground' : 'text-amber-500')}
        aria-hidden="true"
      />
    )
  }

  const name = () => {
    if (!onChoice || !removal.path) return removal.worktree
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

  const reasons = (): ReactNode[] =>
    blockers.map((blocker) => (
      <span key={blocker} className="flex items-center gap-1.5">
        {blockerWords[blocker](removal)}
        {blocker === 'open_pull_request' && removal.pull_request && (
          <a href={removal.pull_request} target="_blank" rel="noreferrer" aria-label="Open the pull request">
            <ExternalLink className="size-3.5" />
          </a>
        )}
      </span>
    ))

  const consequences = () => [
    ...(risky ? blockers.map((blocker) => lostWords[blocker](removal)) : []),
    ...moves.map((move) =>
      reparent ? `${move.branch} moves onto ${move.to}` : `${move.branch} stays on ${move.from}, orphaned`,
    ),
  ]

  const lines = onChoice ? reasons() : consequences()
  const odd = exception()

  return (
    <Item role="listitem" aria-label={removal.worktree} className="flex-col items-stretch gap-1 rounded-none px-4 py-3">
      <div className="flex items-center gap-3">
        {glyph()}
        <ItemTitle className={cn('min-w-0 flex-1', kept && 'text-muted-foreground')}>{name()}</ItemTitle>
        {onChoice && risky && (
          <ToggleGroup
            type="single"
            variant="outline"
            size="sm"
            spacing={0}
            aria-label={`What to do with ${removal.worktree}`}
            value={choice ?? ''}
            onValueChange={(value) => {
              if (value === 'keep' || value === 'remove') onChoice(value)
            }}
          >
            <ToggleGroupItem value="keep">{choiceWords.keep}</ToggleGroupItem>
            <ToggleGroupItem
              value="remove"
              className="data-[state=on]:bg-destructive/15 data-[state=on]:text-destructive"
            >
              {choiceWords.remove}
            </ToggleGroupItem>
          </ToggleGroup>
        )}
        {!onChoice && risky && <span className="text-xs text-muted-foreground">Removed anyway</span>}
      </div>
      {(lines.length > 0 || odd) && (
        <div
          className={cn(
            'flex flex-col gap-1 pl-7',
            onChoice ? 'text-sm' : 'text-xs text-muted-foreground',
            onChoice && kept && 'text-muted-foreground',
          )}
        >
          {lines.map((line) => (typeof line === 'string' ? <span key={line}>{line}</span> : line))}
          {odd && <span className="text-xs text-muted-foreground">{odd}</span>}
        </div>
      )}
    </Item>
  )
}
