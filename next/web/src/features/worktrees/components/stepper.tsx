import { Check, Minus } from 'lucide-react'
import { Fragment, type ReactNode } from 'react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

export type StepState = 'done' | 'now' | 'todo' | 'skipped'

const stepLook: Record<StepState, string> = {
  now: 'border-foreground bg-foreground text-background',
  done: 'border-foreground text-foreground',
  skipped: 'text-muted-foreground',
  todo: 'text-muted-foreground',
}

const mark = (state: StepState, n: number): ReactNode => {
  if (state === 'done') return <Check className="size-3.5" />
  if (state === 'skipped') return <Minus className="size-3.5" />
  return n
}

// a skipped step is its label alone, greyed, so the stepper holds one line; why it was skipped is its tooltip
export function Stepper({ steps }: { steps: { title: string; state: StepState }[] }) {
  return (
    <ol className="flex items-center gap-3" aria-label="Steps">
      {steps.map((step, i) => (
        <Fragment key={step.title}>
          {i > 0 && <li aria-hidden="true" className="h-px flex-1 bg-border" />}
          <li
            className="flex shrink-0 items-center gap-2"
            aria-current={step.state === 'now' ? 'step' : undefined}
            aria-label={step.state === 'skipped' ? `${step.title}, not asked` : undefined}
          >
            <span
              className={cn(
                'flex size-6 items-center justify-center rounded-full border text-xs font-semibold',
                stepLook[step.state],
              )}
            >
              {mark(step.state, i + 1)}
            </span>
            {step.state === 'skipped' ? (
              <Tooltip>
                <TooltipTrigger asChild>
                  {/* biome-ignore lint/a11y/noNoninteractiveTabindex: why it was skipped shows to a keyboard as to a pointer */}
                  <span tabIndex={0} className="text-sm text-muted-foreground">
                    {step.title}
                  </span>
                </TooltipTrigger>
                <TooltipContent>Not asked</TooltipContent>
              </Tooltip>
            ) : (
              <span className={cn('text-sm', step.state === 'now' ? 'font-medium' : 'text-muted-foreground')}>
                {step.title}
              </span>
            )}
          </li>
        </Fragment>
      ))}
    </ol>
  )
}
