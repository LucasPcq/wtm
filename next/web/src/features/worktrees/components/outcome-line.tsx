import { ChevronRight, CircleCheck, CircleMinus, CircleX, type LucideIcon, TriangleAlert } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { CopyFollowUp } from '@/features/worktrees/components/copy-follow-up'
import { CopyPath } from '@/features/worktrees/components/copy-path'
import { StepOutput } from '@/features/worktrees/components/step-output'
import type { OutcomeRow, OutcomeTone } from '@/features/worktrees/utils/outcome'
import { cn } from '@/lib/utils'

const glyph: Record<OutcomeTone, { icon: LucideIcon; className: string }> = {
  ok: { icon: CircleCheck, className: 'text-green-500' },
  attention: { icon: TriangleAlert, className: 'text-amber-500' },
  failed: { icon: CircleX, className: 'text-destructive' },
  stopped: { icon: CircleMinus, className: 'text-muted-foreground' },
}

// one line per thing; what explains a problem, its output or its fix, folds under it until asked for
export function OutcomeLine({ row }: { row: OutcomeRow }) {
  const [open, setOpen] = useState(false)
  const { icon: Icon, className } = glyph[row.tone]
  const foldable = row.output !== undefined || row.followUp !== undefined

  return (
    <Collapsible asChild open={open} onOpenChange={setOpen}>
      <li className="flex flex-col gap-1">
        <div className="flex min-h-8 items-center gap-3 text-sm">
          <Icon className={cn('size-4 shrink-0', className)} aria-hidden="true" />
          {row.title && <span className="shrink-0 font-medium">{row.title}</span>}
          <span className={cn('min-w-0 flex-1', row.title && 'text-muted-foreground')}>{row.message}</span>
          {row.go && <CommandLine followUp={row.go} inline />}
          {row.followUp && !open && <CopyFollowUp followUp={row.followUp} />}
          {foldable && (
            <CollapsibleTrigger asChild>
              <Button variant="ghost" size="icon-sm" aria-label={open ? 'Hide the details' : 'Show the details'}>
                <ChevronRight
                  className={cn('transition-transform motion-reduce:transition-none', open && 'rotate-90')}
                />
              </Button>
            </CollapsibleTrigger>
          )}
        </div>
        {row.path && (
          <div className="pl-7 text-xs">
            <CopyPath path={row.path} />
          </div>
        )}
        {foldable && (
          <CollapsibleContent className="flex flex-col gap-2 pl-7">
            {row.output && <StepOutput lines={row.output.lines} log={row.output.log} />}
            {row.followUp && <CommandLine followUp={row.followUp} />}
          </CollapsibleContent>
        )}
      </li>
    </Collapsible>
  )
}
