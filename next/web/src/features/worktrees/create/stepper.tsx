import { Check } from 'lucide-react'
import { Fragment } from 'react'
import { cn } from '@/lib/utils'

export type StepState = 'done' | 'now' | 'todo' | 'skipped'

// Stepper is where the dialog stands among its steps: done, the current one, those to come, one the core asks
// nothing of passed as not asked.
export function Stepper({ steps }: { steps: { title: string; state: StepState }[] }) {
  return (
    <ol className="flex items-center gap-3" aria-label="Steps">
      {steps.map((step, i) => (
        <Fragment key={step.title}>
          {i > 0 && <li aria-hidden="true" className="h-px flex-1 bg-border" />}
          <li className="flex items-center gap-2" aria-current={step.state === 'now' ? 'step' : undefined}>
            <span
              className={cn(
                'flex size-6 items-center justify-center rounded-full border text-xs font-semibold',
                step.state === 'todo' ? 'text-muted-foreground' : 'border-primary bg-primary text-primary-foreground',
              )}
            >
              {step.state === 'done' || step.state === 'skipped' ? <Check className="size-3.5" /> : i + 1}
            </span>
            <span className={cn('text-sm', step.state === 'now' ? 'font-medium' : 'text-muted-foreground')}>
              {step.title}
              {step.state === 'skipped' && ' · not asked'}
            </span>
          </li>
        </Fragment>
      ))}
    </ol>
  )
}
