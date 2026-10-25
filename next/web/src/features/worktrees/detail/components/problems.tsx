import { TriangleAlert } from 'lucide-react'
import type { StatusProblem } from '@/api/types.gen'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { Glyph } from '@/features/worktrees/components/glyph'
import { sentence } from '@/features/worktrees/utils/sentence'

export function Problems({ problems: [first, ...more] }: { problems: StatusProblem[] }) {
  if (!first) return null

  const fix = ({ message, fix }: StatusProblem) => (
    <div className="flex flex-col gap-2">
      <Glyph icon={TriangleAlert} tone="attention" className="font-medium">
        {sentence(message)}
      </Glyph>
      {fix && (
        <>
          <p className="text-sm text-muted-foreground">
            {sentence(fix.message)}. Not in the web interface yet: copy this into a terminal.
          </p>
          <CommandLine followUp={fix} />
        </>
      )}
    </div>
  )

  return (
    <section aria-label="To do" className="flex flex-col gap-3 rounded-lg border bg-card p-4">
      {fix(first)}
      {more.length > 0 && (
        <details className="group text-sm">
          <summary className="cursor-pointer text-muted-foreground">
            {more.length} more {more.length === 1 ? 'problem' : 'problems'}
          </summary>
          <div className="mt-3 flex flex-col gap-4">
            {more.map((problem) => (
              <div key={problem.code + problem.message}>{fix(problem)}</div>
            ))}
          </div>
        </details>
      )}
    </section>
  )
}
