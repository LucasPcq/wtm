import { ChevronRight } from 'lucide-react'
import type { MutationCommand } from '@/api/queries/commands'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Timeline } from '@/features/worktrees/components/timeline'
import { doneOf, type Progress, took } from '@/features/worktrees/utils/progress'

// a run that went well folds its steps into one line; one under way, failed or cancelled shows them all
export function RunSteps<C extends MutationCommand>({ progress, ok }: { progress: Progress<C>; ok: boolean }) {
  if (!ok) return <Timeline units={progress.units} />

  return (
    <Collapsible>
      <CollapsibleTrigger asChild>
        <Button variant="ghost" size="sm" className="-ml-2 text-muted-foreground [&[data-state=open]>svg]:rotate-90">
          <ChevronRight className="transition-transform motion-reduce:transition-none" />
          {[`${doneOf(progress).total} steps`, took(progress)].filter(Boolean).join(' · ')}
        </Button>
      </CollapsibleTrigger>
      <CollapsibleContent className="pt-3">
        <Timeline units={progress.units} />
      </CollapsibleContent>
    </Collapsible>
  )
}
