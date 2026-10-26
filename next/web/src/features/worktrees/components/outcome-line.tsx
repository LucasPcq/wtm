import { ChevronRight, CircleCheck, CircleMinus, CircleX, type LucideIcon, TriangleAlert } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { CopyPath } from '@/features/worktrees/components/copy-path'
import { toneIcon } from '@/features/worktrees/components/glyph'
import { StepOutput } from '@/features/worktrees/components/step-output'
import type { OutcomeRow, OutcomeTone } from '@/features/worktrees/utils/outcome'
import { cn } from '@/lib/utils'

const glyph: Record<OutcomeTone, LucideIcon> = {
  ok: CircleCheck,
  attention: TriangleAlert,
  failed: CircleX,
  muted: CircleMinus,
}

export function OutcomeLine({ row }: { row: OutcomeRow }) {
  const [open, setOpen] = useState(false)
  const Icon = glyph[row.tone ?? 'muted']
  const foldable = row.output !== undefined || row.followUp !== undefined

  return (
    <Collapsible asChild open={open} onOpenChange={setOpen}>
      <li className="flex flex-col gap-1">
        <div className="flex min-h-8 items-center gap-3 text-sm">
          {row.tone ? (
            <Icon className={cn(toneIcon({ tone: row.tone }), 'size-4')} aria-hidden="true" />
          ) : (
            <span className="size-4 shrink-0" />
          )}
          {row.title && <span className="shrink-0 font-medium">{row.title}</span>}
          <span className={cn('min-w-0 flex-1', row.title && 'text-muted-foreground')}>{row.message}</span>
          {row.go && <CommandLine followUp={row.go} look="inline" />}
          {row.followUp && !open && <CommandLine followUp={row.followUp} look="copy" />}
          {foldable && (
            <CollapsibleTrigger asChild>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={`Details of ${[row.title, row.message].filter(Boolean).join(': ')}`}
              >
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
