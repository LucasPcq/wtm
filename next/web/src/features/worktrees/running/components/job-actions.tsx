import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import type { JobActs } from '@/features/worktrees/running/hooks/use-job-actions'
import { actLook } from '@/features/worktrees/running/utils/job-words'

// aria-disabled, not disabled: a disabled button takes no pointer, so its reason would never show
export function JobActions({ acts, job }: { acts: JobActs['acts']; job: string }) {
  return acts.map(({ act, why, run }) => {
    const { icon: Icon, label } = actLook[act]
    return (
      <Tooltip key={act}>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="sm"
            aria-label={`${label} ${job}`}
            aria-disabled={why !== undefined}
            className="aria-disabled:cursor-not-allowed aria-disabled:opacity-50"
            onClick={() => why === undefined && run()}
          >
            <Icon />
            {label}
          </Button>
        </TooltipTrigger>
        {why && <TooltipContent>{why}</TooltipContent>}
      </Tooltip>
    )
  })
}
