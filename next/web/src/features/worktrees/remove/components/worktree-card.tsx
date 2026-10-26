import { CircleAlert } from 'lucide-react'
import type { ReactNode } from 'react'
import type { CleanMove, CleanRemoval } from '@/api/types.gen'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Path } from '@/features/worktrees/components/path'
import { type Choice, unsafe } from '@/features/worktrees/remove/utils/plan'
import { blockerWords, lostWords } from '@/features/worktrees/remove/utils/words'
import { cn } from '@/lib/utils'

const choiceWords: Record<Choice, string> = { keep: 'Keep it', remove: 'Remove anyway' }

export function WorktreeCard({
  removal,
  moves = [],
  reparent = false,
  choice,
  onChoice,
}: {
  removal: CleanRemoval
  moves?: CleanMove[]
  reparent?: boolean
  choice?: Choice | undefined
  onChoice?: (choice: Choice) => void
}) {
  const blockers = removal.blockers ?? []
  const asked = onChoice && unsafe(removal)

  const branch = () => {
    if (!removal.path) return `${removal.worktree}, a branch with no worktree, deleted alone`
    if (!removal.branch) return 'On no branch: only the worktree goes'
    return `${removal.branch}, deleted with it`
  }

  const note = () => {
    if (choice === 'remove') return `Removed anyway: ${blockers.map((blocker) => lostWords[blocker]).join(', ')}.`
    if (choice === 'keep') return 'Kept: it stays as it is.'
    return "Choose: it can't be removed safely."
  }

  const badge = () => {
    if (!unsafe(removal)) return <span className="text-xs text-muted-foreground">Safe to remove</span>
    return (
      <span className="flex items-center gap-1 text-xs text-destructive">
        <CircleAlert className="size-3" />
        {onChoice ? 'Unsafe' : 'Removed anyway'}
      </span>
    )
  }

  return (
    <section
      aria-label={removal.worktree}
      className={cn('flex flex-col gap-2.5 rounded-lg border p-3.5', unsafe(removal) && 'border-destructive')}
    >
      <div className="flex flex-col gap-0.5">
        <div className="flex items-center gap-2">
          <h4 className="min-w-0 flex-1 truncate text-sm font-medium">{removal.worktree}</h4>
          {badge()}
        </div>
        {removal.path && <Path path={removal.path} />}
      </div>

      <dl className="grid grid-cols-[5.5rem_1fr] gap-x-4 gap-y-1.5 text-sm">
        <Row label="Branch">{branch()}</Row>
        {blockers.length > 0 && (
          <Row label="Holds">
            {blockers.map((blocker) => (
              <span key={blocker} className="block">
                {blockerWords[blocker](removal)}
              </span>
            ))}
          </Row>
        )}
        {(removal.hooks ?? []).length > 0 && (
          <Row label="Before">
            Runs on_clean
            {removal.hooks?.map((hook) => (
              <code key={hook} className="block font-mono text-xs text-muted-foreground">
                {hook}
              </code>
            ))}
          </Row>
        )}
        {moves.length > 0 && (
          <Row label="Children">
            {moves.map((move) => (
              <span key={move.branch} className="block">
                {reparent ? `${move.branch} moves onto ${move.to}` : `${move.branch} stays on it, orphaned`}
              </span>
            ))}
          </Row>
        )}
        {(removal.namespaces ?? []).length > 0 && (
          <Row label="Data">
            {removal.namespaces?.join(', ')} stay in their services
            <span className="block text-xs text-muted-foreground">wtm-next does not drop them yet</span>
          </Row>
        )}
      </dl>

      {asked && (
        <div className="flex flex-col gap-2 border-t pt-3">
          <RadioGroup
            aria-label={`What to do with ${removal.worktree}`}
            value={choice ?? ''}
            onValueChange={(value) => onChoice(value === 'remove' ? 'remove' : 'keep')}
            className="grid grid-cols-2 gap-2"
          >
            {(['keep', 'remove'] as const).map((value) => (
              <Label
                key={value}
                htmlFor={`${removal.worktree}-${value}`}
                className={cn(
                  'rounded-md border px-3 py-2 font-normal',
                  choice === value && (value === 'remove' ? 'border-destructive text-destructive' : 'border-primary'),
                )}
              >
                <RadioGroupItem id={`${removal.worktree}-${value}`} value={value} />
                {choiceWords[value]}
              </Label>
            ))}
          </RadioGroup>
          <p className={cn('text-xs', choice === 'remove' ? 'text-destructive' : 'text-muted-foreground')}>{note()}</p>
        </div>
      )}
    </section>
  )
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0">{children}</dd>
    </>
  )
}
