import { Check, LoaderCircle, X } from 'lucide-react'
import type { ReactNode } from 'react'
import { StepOutput } from '@/features/worktrees/components/step-output'
import { phaseLabel } from '@/features/worktrees/utils/phases'
import { type PhaseRun, seconds, type Unit } from '@/features/worktrees/utils/progress'
import { cn } from '@/lib/utils'

const marker: Record<PhaseRun['state'], ReactNode> = {
  todo: <span className="size-1.5 rounded-full bg-muted-foreground" />,
  now: <LoaderCircle className="size-3 animate-spin motion-reduce:animate-none" />,
  done: <Check className="size-3 text-green-500" />,
  failed: <X className="size-3 text-destructive" />,
}

// once a run ended, its failed step's output is shown with the failure, not again among its steps
export function Timeline({ units, settled = false }: { units: Unit[]; settled?: boolean }) {
  const named = units.length > 1

  return (
    <div className="flex flex-col gap-4">
      {units.map(({ subject, phases, lines }) => (
        <section key={subject} aria-label={subject}>
          {named && <h3 className="mb-2 font-mono text-xs text-muted-foreground">{subject}</h3>}
          <ol className="flex flex-col">
            {phases.map((run, i) => (
              <li key={run.phase} className="flex gap-3">
                <div className="flex w-5 flex-col items-center">
                  <span
                    className={cn(
                      'flex size-5 shrink-0 items-center justify-center rounded-full',
                      run.state === 'todo' ? 'border' : 'bg-muted',
                    )}
                  >
                    {marker[run.state]}
                  </span>
                  {i < phases.length - 1 && <span className="w-px flex-1 bg-border" />}
                </div>

                <div className="flex min-w-0 flex-1 flex-col gap-2 pb-3">
                  <div className="flex items-center text-sm">
                    <span
                      className={cn(
                        'flex-1',
                        run.state === 'todo' && 'text-muted-foreground',
                        run.state === 'now' && 'font-medium',
                      )}
                    >
                      {phaseLabel[run.phase]}
                    </span>
                    <span className="text-xs text-muted-foreground tabular-nums">{seconds(run)}</span>
                  </div>
                  {(run.state === 'now' || (run.state === 'failed' && !settled)) && lines.length > 0 && (
                    <StepOutput lines={lines} log={run.log} />
                  )}
                </div>
              </li>
            ))}
          </ol>
        </section>
      ))}
    </div>
  )
}
