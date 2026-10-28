import { MoveRight, TriangleAlert } from 'lucide-react'
import type { TeardownMove } from '@/api/types.gen'
import { ChoiceOptions } from '@/components/choice-options'
import { Item } from '@/components/ui/item'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { ItemList } from '@/features/worktrees/components/item-list'

export function ChildrenStep({
  moves,
  reparent,
  onReparent,
}: {
  moves: TeardownMove[]
  reparent: boolean
  onReparent: (on: boolean) => void
}) {
  const [only] = moves
  const one = moves.length === 1 ? only : undefined

  const options = () => {
    if (one)
      return [
        { value: 'move', title: `Move it onto ${one.to}`, description: `It rebases onto ${one.to} on its next sync.` },
        { value: 'leave', title: `Leave it on ${one.from}`, description: 'Move it later with wtm reparent.' },
      ]
    return [
      {
        value: 'move',
        title: 'Move each onto the nearest worktree that stays',
        description: 'They rebase onto it on their next sync.',
      },
      { value: 'leave', title: 'Leave them where they are', description: 'Move them later with wtm reparent.' },
    ]
  }

  const target = (move: TeardownMove) => {
    if (!move.base) return <span>{move.to}</span>
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          {/* biome-ignore lint/a11y/noNoninteractiveTabindex: why it lands there shows to a keyboard as to a pointer */}
          <span tabIndex={0}>{move.to}</span>
        </TooltipTrigger>
        <TooltipContent>The base branch: no worktree it was stacked on stays.</TooltipContent>
      </Tooltip>
    )
  }

  return (
    <>
      <p className="text-sm">
        {one
          ? `${one.branch} is stacked on ${one.from}, which you're removing.`
          : `${moves.length} worktrees are stacked on worktrees you're removing.`}
      </p>

      <ChoiceOptions
        id="reparent"
        label="What becomes of the children"
        value={reparent ? 'move' : 'leave'}
        options={options()}
        onChange={(value) => onReparent(value === 'move')}
      />

      {!one && (
        <section aria-label="Where each goes" className="flex flex-col gap-2">
          <h3 className="text-xs font-medium text-muted-foreground">Where each goes</h3>
          <ItemList>
            {moves.map((move) => (
              <Item
                key={move.branch}
                role="listitem"
                aria-label={move.branch}
                className="gap-3 rounded-none px-4 py-2.5"
              >
                <span className="w-44 shrink-0 truncate font-medium">{move.branch}</span>
                {reparent ? (
                  <span className="flex min-w-0 items-center gap-1.5">
                    <span className="text-muted-foreground">{move.from}</span>
                    <MoveRight className="size-3.5 text-muted-foreground" aria-label="onto" />
                    {target(move)}
                  </span>
                ) : (
                  <span className="flex items-center gap-1.5 text-muted-foreground">
                    <TriangleAlert className="size-3.5 text-amber-500" aria-hidden="true" />
                    stays on {move.from}
                  </span>
                )}
              </Item>
            ))}
          </ItemList>
        </section>
      )}
    </>
  )
}
