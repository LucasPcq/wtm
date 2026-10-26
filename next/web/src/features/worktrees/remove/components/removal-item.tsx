import { CircleX, ExternalLink, type LucideIcon, TriangleAlert } from 'lucide-react'
import type { ReactNode } from 'react'
import type { CleanMove, CleanRemoval } from '@/api/types.gen'
import { Item, ItemTitle } from '@/components/ui/item'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { type Choice, unsafe } from '@/features/worktrees/remove/utils/plan'
import { blockerWords, choiceWords, lostWords } from '@/features/worktrees/remove/utils/words'
import { cn } from '@/lib/utils'

// kept, an unsafe worktree no longer asks anything of the person: its glyph steps back with its line
const look: Record<'undecided' | 'kept' | 'forced', { icon: LucideIcon; className: string }> = {
  undecided: { icon: TriangleAlert, className: 'text-amber-500' },
  kept: { icon: TriangleAlert, className: 'text-muted-foreground' },
  forced: { icon: CircleX, className: 'text-destructive' },
}

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

  const state = () => {
    if (!onChoice) return 'forced'
    return kept ? 'kept' : 'undecided'
  }

  const glyph = () => {
    const { icon: Icon, className } = look[state()]
    return <Icon className={cn('size-3.5 shrink-0', className)} aria-hidden="true" />
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
        {glyph()}
        {blockerWords[blocker](removal)}
        {blocker === 'open_pull_request' && removal.pull_request && (
          <a href={removal.pull_request} target="_blank" rel="noreferrer" aria-label="Open the pull request">
            <ExternalLink className="size-3.5" />
          </a>
        )}
      </span>
    ))

  const consequences = (): ReactNode[] => [
    ...(risky
      ? blockers.map((blocker) => (
          <span key={blocker} className="flex items-center gap-1.5">
            {glyph()}
            {lostWords[blocker](removal)}
          </span>
        ))
      : []),
    ...moves.map((move) =>
      reparent ? `${move.branch} moves onto ${move.to}` : `${move.branch} stays on ${move.from}, orphaned`,
    ),
  ]

  const lines = onChoice ? reasons() : consequences()
  const odd = exception()

  return (
    <Item role="listitem" aria-label={removal.worktree} className="items-center gap-3 rounded-none px-4 py-3">
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <ItemTitle className={cn('min-w-0', kept && 'text-muted-foreground')}>{name()}</ItemTitle>
        {(lines.length > 0 || odd) && (
          <div className={cn('flex flex-col gap-1 text-xs', (!onChoice || kept) && 'text-muted-foreground')}>
            {lines.map((line) => (typeof line === 'string' ? <span key={line}>{line}</span> : line))}
            {odd && <span className="text-muted-foreground">{odd}</span>}
          </div>
        )}
      </div>
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
      {!onChoice && risky && <span className="shrink-0 text-xs text-muted-foreground">Removed anyway</span>}
    </Item>
  )
}
