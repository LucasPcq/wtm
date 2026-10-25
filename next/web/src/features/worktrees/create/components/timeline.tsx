import { Check, LoaderCircle, X } from 'lucide-react'
import type { ReactNode } from 'react'
import { type PhaseRun, type Progress, seconds } from '@/features/worktrees/create/utils/progress'
import { phaseLabel } from '@/features/worktrees/create/utils/words'
import { copy } from '@/features/worktrees/utils/copy'
import { cn } from '@/lib/utils'

const marker: Record<PhaseRun['state'], ReactNode> = {
  todo: <span className="size-1.5 rounded-full bg-muted-foreground" />,
  now: <LoaderCircle className="size-3 animate-spin motion-reduce:animate-none" />,
  done: <Check className="size-3 text-green-500" />,
  failed: <X className="size-3 text-destructive" />,
}

export function Timeline({ progress }: { progress: Progress }) {
  const named = progress.units.length > 1

  return (
    <div className="flex flex-col gap-4">
      {progress.units.map(({ subject, phases, lines }) => (
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
                  {(run.state === 'now' || run.state === 'failed') && lines.length > 0 && (
                    <div className="rounded-md bg-muted px-3 py-2">
                      <pre
                        // the output follows its last line, as a terminal does
                        ref={(pre) => pre?.scrollTo({ top: pre.scrollHeight })}
                        className="h-28 overflow-y-auto font-mono text-xs whitespace-pre-wrap text-muted-foreground [scrollbar-color:var(--border)_transparent] [scrollbar-width:thin]"
                      >
                        {lines.join('\n')}
                      </pre>
                      {run.log && (
                        <button
                          type="button"
                          className="mt-1 text-xs underline underline-offset-4"
                          onClick={() => run.log && copy(run.log)}
                        >
                          Copy the log's path
                        </button>
                      )}
                    </div>
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
