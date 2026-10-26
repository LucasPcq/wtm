import { copy } from '@/features/worktrees/utils/copy'

export function StepOutput({ lines, log }: { lines: string[]; log?: string | undefined }) {
  return (
    <div className="rounded-md bg-muted px-3 py-2">
      <pre
        // the output follows its last line, as a terminal does
        ref={(pre) => pre?.scrollTo({ top: pre.scrollHeight })}
        className="max-h-28 overflow-y-auto font-mono text-xs whitespace-pre-wrap text-muted-foreground"
      >
        {lines.join('\n')}
      </pre>
      {log && (
        <button type="button" className="mt-1 text-xs underline underline-offset-4" onClick={() => copy(log)}>
          Copy the log's path
        </button>
      )}
    </div>
  )
}
