import { MoveRight, Trash2, TriangleAlert } from 'lucide-react'
import type { CleanMove } from '@/api/types.gen'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'

export function ChildrenStep({
  moves,
  reparent,
  onReparent,
}: {
  moves: CleanMove[]
  reparent: boolean
  onReparent: (on: boolean) => void
}) {
  const parents = [...new Set(moves.map((move) => move.from))]

  const lands = (move: CleanMove) => {
    if (!reparent)
      return (
        <span className="flex items-center gap-1.5 text-muted-foreground">
          <TriangleAlert className="size-3 text-amber-600 dark:text-amber-500" />
          stays on {move.from}, which goes
        </span>
      )
    return (
      <span className="flex items-center gap-1.5">
        <MoveRight className="size-3 text-muted-foreground" />
        {move.to}
        <span className="text-xs text-muted-foreground">{move.base ? 'base branch' : 'nearest that stays'}</span>
      </span>
    )
  }

  return (
    <div className="flex flex-col gap-3">
      <h3 className="font-medium">Children left behind</h3>
      <p className="text-sm text-muted-foreground">
        {moves.length === 1
          ? `${moves[0]?.branch} is stacked on ${moves[0]?.from}, which goes.`
          : `${moves.length} worktrees are stacked on worktrees that go. One choice applies to all of them.`}
      </p>
      <RadioGroup value={reparent ? 'move' : 'leave'} onValueChange={(value) => onReparent(value === 'move')}>
        <Label htmlFor="reparent-move" className="flex items-start gap-2 font-normal">
          <RadioGroupItem id="reparent-move" value="move" className="mt-0.5" />
          <span className="flex flex-col">
            {moves.length === 1
              ? 'Move it onto the nearest worktree that stays'
              : 'Move them onto the nearest worktree that stays'}
            <span className="text-xs text-muted-foreground">
              It rebases onto it on its next sync; the base branch when none stays.
            </span>
          </span>
        </Label>
        <Label htmlFor="reparent-leave" className="flex items-start gap-2 font-normal">
          <RadioGroupItem id="reparent-leave" value="leave" className="mt-0.5" />
          <span className="flex flex-col">
            {moves.length === 1 ? 'Leave it where it is' : 'Leave them where they are'}
            <span className="text-xs text-muted-foreground">
              It keeps pointing at a removed parent; move it later with wtm reparent.
            </span>
          </span>
        </Label>
      </RadioGroup>

      <section className="flex flex-col gap-3 pt-1" aria-label="Where each child goes">
        {parents.map((parent) => (
          <section key={parent} aria-label={`Under ${parent}`} className="flex flex-col">
            <h4 className="flex items-center gap-1.5 text-[13px] font-medium">
              <Trash2 className="size-3 text-destructive" />
              {parent}
              <span className="text-xs font-normal text-muted-foreground">goes</span>
            </h4>
            {moves
              .filter((move) => move.from === parent)
              .map((move) => (
                <div key={move.branch} className="flex items-center gap-2 border-b py-1.5 text-sm">
                  <span className="flex-1 font-medium">{move.branch}</span>
                  {lands(move)}
                </div>
              ))}
          </section>
        ))}
      </section>
    </div>
  )
}
