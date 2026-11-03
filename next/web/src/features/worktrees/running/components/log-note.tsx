import { Info } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { LogDispatch, LogView } from '@/features/worktrees/running/hooks/use-log-view'
import { instantOn } from '@/features/worktrees/running/utils/log-search'

// why the lines are what they are: a search of text, a jump and its reason, the whole log read; a job's output has no
// level, so every filter here says it is one of text
export function LogNote({ view, dispatch, lines }: { view: LogView; dispatch: LogDispatch; lines: { at?: string }[] }) {
  const searching = view.query.text !== ''
  const first = lines.find((line) => line.at)?.at

  const said = () => {
    if (view.whole && 'search' in view.whole) return 'The whole log, its rotated files included: the lines that match.'
    if (view.whole) return 'The log file, from that time on.'
    switch (view.note?.kind) {
      case 'error':
        return 'Latest error: the last line matching error, fail, panic or exception. A text match: the job’s output has no level.'
      case 'no-error':
        return 'No line here says error, fail, panic or exception.'
      case 'time':
        return `The first line from ${view.note.time}.`
      case 'after':
        return `No line at or after ${view.note.time}.`
      case 'older':
        return `The lines before ${view.note.time} are in the log file.`
    }
    if (view.only && searching)
      return `Only the lines matching ${view.query.text}: a text match, the job prints to a terminal and its lines carry no level.`
    return undefined
  }

  const action = () => {
    if (view.whole)
      return (
        <Button
          variant="link"
          size="xs"
          className="h-auto p-0"
          onClick={() => dispatch({ type: 'whole', whole: undefined })}
        >
          Back to the last lines
        </Button>
      )
    if (view.note?.kind === 'older' && first) {
      const since = new Date(instantOn(first, view.note.time)).toISOString()
      return (
        <Button
          variant="link"
          size="xs"
          className="h-auto p-0"
          onClick={() => dispatch({ type: 'whole', whole: { since } })}
        >
          Read them
        </Button>
      )
    }
    if (searching)
      return (
        <Button
          variant="link"
          size="xs"
          className="h-auto p-0"
          onClick={() => dispatch({ type: 'whole', whole: { search: view.query } })}
        >
          Search the whole log
        </Button>
      )
    return undefined
  }

  const text = said()
  const act = action()
  if (!text && !act) return null

  return (
    <p className="flex items-center gap-2 px-6 pb-2 text-xs text-muted-foreground">
      <Info className="size-3.5 shrink-0" aria-hidden="true" />
      {text && <span className="min-w-0">{text}</span>}
      {act}
    </p>
  )
}
