import { ChevronRight } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { toneIcon } from '@/features/worktrees/components/glyph'
import { StepOutput } from '@/features/worktrees/components/step-output'
import { terminalLine } from '@/features/worktrees/exec/utils/output'
import { phaseLook } from '@/features/worktrees/utils/phase-look'
import type { PhaseRun } from '@/features/worktrees/utils/progress'
import { cn } from '@/lib/utils'

// one step of a run under way, a worktree's command or a job: folded, its lines live behind its chevron
export function PhaseRow({ name, run, label, lines }: { name: string; run: PhaseRun; label: string; lines: string[] }) {
  const [open, setOpen] = useState(false)
  const { icon: Icon, tone } = phaseLook[run.state]

  return (
    <Collapsible asChild open={open} onOpenChange={setOpen}>
      <li aria-label={name} className="flex flex-col gap-1">
        <div className="flex min-h-8 items-center gap-3 text-sm">
          <Icon className={cn(toneIcon({ tone }), 'size-4')} aria-hidden="true" />
          <span className="shrink-0 font-medium">{name}</span>
          <span className="min-w-0 flex-1 truncate text-muted-foreground">{label}</span>
          {run.state !== 'todo' && (
            <CollapsibleTrigger asChild>
              <Button variant="ghost" size="icon-sm" aria-label={`Output of ${name}`}>
                <ChevronRight
                  className={cn('transition-transform motion-reduce:transition-none', open && 'rotate-90')}
                />
              </Button>
            </CollapsibleTrigger>
          )}
        </div>
        <CollapsibleContent className="pl-7">
          {open && (
            <>
              <StepOutput lines={lines.map(terminalLine)} log={run.log} />
              {run.state === 'now' && (
                <p className="mt-1 text-xs text-muted-foreground">Following its output · the last lines</p>
              )}
            </>
          )}
        </CollapsibleContent>
      </li>
    </Collapsible>
  )
}
