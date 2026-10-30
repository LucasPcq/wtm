import { CircleCheck, CircleMinus, CircleX, LoaderCircle, type LucideIcon } from 'lucide-react'
import { type Tone, toneIcon } from '@/features/worktrees/components/glyph'
import { cn } from '@/lib/utils'

export type UnitState = 'todo' | 'now' | 'done' | 'failed'

const stateLook: Record<UnitState, { icon: LucideIcon; tone: Tone }> = {
  todo: { icon: CircleMinus, tone: 'muted' },
  now: { icon: LoaderCircle, tone: 'busy' },
  done: { icon: CircleCheck, tone: 'ok' },
  failed: { icon: CircleX, tone: 'failed' },
}

// the worktrees of a run while it runs, one line each: where it stands, in the command's words
export function RunningUnits({ rows }: { rows: { subject: string; state: UnitState; words: string }[] }) {
  return (
    <ul aria-label="Worktrees" className="flex flex-col gap-1">
      {rows.map(({ subject, state, words }) => {
        const { icon: Icon, tone } = stateLook[state]
        return (
          <li key={subject} className="flex min-h-8 items-center gap-3 text-sm">
            <Icon className={cn(toneIcon({ tone }), 'size-4')} aria-hidden="true" />
            <span className="font-medium">{subject}</span>
            <span className="text-muted-foreground">{words}</span>
          </li>
        )
      })}
    </ul>
  )
}
