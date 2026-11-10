import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { PartMark } from '@/features/run-setup/components/part-mark'
import type { PartState } from '@/features/run-setup/utils/setup'
import { type Part, partWords } from '@/features/run-setup/utils/words'
import { cn } from '@/lib/utils'

// a part folded into one line: answered, its summary and Edit; skipped, why on hover; not reached, greyed
export function PartRow({
  part,
  number,
  state,
  summary,
  waiting,
  skippedWhy,
  onEdit,
}: {
  part: Part
  number: number
  state: PartState
  summary: string
  waiting: string | undefined
  skippedWhy: string
  onEdit: (() => void) | undefined
}) {
  const line = () => {
    if (state === 'skipped')
      return (
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="text-sm text-muted-foreground underline decoration-dotted">Skipped</span>
          </TooltipTrigger>
          <TooltipContent>{skippedWhy}</TooltipContent>
        </Tooltip>
      )
    if (waiting) return <span className="text-sm text-muted-foreground">After {waiting}</span>
    return <span className="text-sm text-muted-foreground">{summary}</span>
  }

  return (
    <section
      aria-label={partWords[part].title}
      className={cn(
        'flex items-center gap-3 rounded-lg border px-5 py-4',
        (waiting || state === 'skipped') && 'opacity-60',
      )}
    >
      <PartMark state={state} number={number} />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <h2 className="font-medium">{partWords[part].title}</h2>
        {line()}
      </div>
      {onEdit && (
        <Button variant="ghost" onClick={onEdit} aria-label={`Edit ${partWords[part].title}`}>
          Edit
        </Button>
      )}
    </section>
  )
}
