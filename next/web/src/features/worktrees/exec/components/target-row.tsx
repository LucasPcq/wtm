import { ChevronRight, CircleCheck, CircleDashed, CircleX, LoaderCircle, type LucideIcon } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { type Tone, toneIcon } from '@/features/worktrees/components/glyph'
import { StepOutput } from '@/features/worktrees/components/step-output'
import { duration, elapsed, terminalLine } from '@/features/worktrees/exec/utils/output'
import type { PhaseRun, Unit } from '@/features/worktrees/utils/progress'
import { cn } from '@/lib/utils'

const look: Record<PhaseRun['state'], { icon: LucideIcon; tone: Tone }> = {
  todo: { icon: CircleDashed, tone: 'muted' },
  now: { icon: LoaderCircle, tone: 'busy' },
  done: { icon: CircleCheck, tone: 'ok' },
  failed: { icon: CircleX, tone: 'failed' },
}

// one worktree while the command runs: folded, its live output behind its chevron
export function TargetRow({ unit, now, cancelling }: { unit: Unit; now: number; cancelling: boolean }) {
  const [open, setOpen] = useState(false)
  const run = unit.phases[0]
  const state = run?.state ?? 'todo'
  const { icon: Icon, tone } = look[state]

  const said = () => {
    if (state === 'todo') return cancelling ? "won't start" : 'queued'
    if (state === 'now') return cancelling || !run?.startedAt ? 'stopping' : elapsed(run.startedAt, now)
    const took = run?.startedAt && run.endedAt ? duration(Date.parse(run.endedAt) - Date.parse(run.startedAt)) : ''
    return state === 'failed' ? ['failed', took].filter(Boolean).join(' · ') : took
  }

  return (
    <Collapsible asChild open={open} onOpenChange={setOpen}>
      <li aria-label={unit.subject} className="flex flex-col gap-1">
        <div className="flex min-h-8 items-center gap-3 text-sm">
          <Icon className={cn(toneIcon({ tone }), 'size-4')} aria-hidden="true" />
          <span className="shrink-0 font-medium">{unit.subject}</span>
          <span className="min-w-0 flex-1 text-muted-foreground">{said()}</span>
          {state !== 'todo' && (
            <CollapsibleTrigger asChild>
              <Button variant="ghost" size="icon-sm" aria-label={`Output of ${unit.subject}`}>
                <ChevronRight
                  className={cn('transition-transform motion-reduce:transition-none', open && 'rotate-90')}
                />
              </Button>
            </CollapsibleTrigger>
          )}
        </div>
        <CollapsibleContent className="pl-7">
          {open && <StepOutput lines={unit.lines.map(terminalLine)} log={run?.log} />}
        </CollapsibleContent>
      </li>
    </Collapsible>
  )
}
