import { Check, Minus, TriangleAlert } from 'lucide-react'
import type { PartState } from '@/features/run-setup/utils/setup'
import { cn } from '@/lib/utils'

export const stateWords: Record<PartState, string> = {
  to_do: 'To do',
  now: 'In progress',
  done: 'Done',
  to_check: 'To check',
  skipped: 'Skipped',
}

export function PartMark({ state, number }: { state: PartState; number: number }) {
  const circle = 'flex size-5 shrink-0 items-center justify-center rounded-full text-[11px]'

  if (state === 'done')
    return (
      <span className={cn(circle, 'border border-emerald-500/40 text-emerald-500')} role="img" aria-label="Done">
        <Check className="size-3" />
      </span>
    )
  if (state === 'to_check')
    return <TriangleAlert className="size-5 shrink-0 text-amber-500" role="img" aria-label="To check" />
  if (state === 'skipped')
    return (
      <span className={cn(circle, 'border text-muted-foreground')} role="img" aria-label="Skipped">
        <Minus className="size-3" />
      </span>
    )
  if (state === 'now') return <span className={cn(circle, 'bg-foreground font-medium text-background')}>{number}</span>
  return <span className={cn(circle, 'border text-muted-foreground')}>{number}</span>
}
