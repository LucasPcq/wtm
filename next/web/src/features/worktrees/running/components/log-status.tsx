import { Clock, Keyboard, WrapText } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Switch } from '@/components/ui/switch'
import type { LogDispatch, LogView } from '@/features/worktrees/running/hooks/use-log-view'
import { cn } from '@/lib/utils'

const shortcuts: [string, string][] = [
  ['Search this log', '/'],
  ['Next match', '↵'],
  ['Previous match', '⇧ ↵'],
  ['Match case', '⌥ C'],
  ['Regex', '⌥ R'],
  ['Only matching', '⌥ M'],
  ['Latest error', 'E'],
  ['Go to time', 'T'],
  ['Follow', 'F'],
  ['Wrap lines', 'W'],
  ['Close the search', 'Esc'],
]

export function LogStatus({
  status,
  view,
  dispatch,
  canFollow,
}: {
  status: string
  view: LogView
  dispatch: LogDispatch
  canFollow: boolean
}) {
  return (
    <footer className="flex items-center gap-3 border-t px-6 py-2 text-xs text-muted-foreground">
      <span className="min-w-0 flex-1 truncate">{status}</span>
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label="Wrap lines"
        aria-pressed={view.wrap}
        className={cn(view.wrap && 'bg-secondary')}
        onClick={() => dispatch({ type: 'wrap' })}
      >
        <WrapText />
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label="Show the times"
        aria-pressed={view.stamps}
        className={cn(view.stamps && 'bg-secondary')}
        onClick={() => dispatch({ type: 'stamps' })}
      >
        <Clock />
      </Button>
      <div className="flex items-center gap-2">
        <Switch
          id="follow"
          checked={view.follow}
          disabled={!canFollow}
          onCheckedChange={(follow) => dispatch({ type: 'follow', follow })}
        />
        <Label htmlFor="follow" className="text-xs font-normal text-muted-foreground">
          Follow
        </Label>
      </div>
      <Popover>
        <PopoverTrigger asChild>
          <Button variant="ghost" size="icon-sm" aria-label="Keyboard">
            <Keyboard />
          </Button>
        </PopoverTrigger>
        <PopoverContent align="end" side="top" className="w-60 p-1" aria-label="Keyboard">
          <p className="px-2 py-1.5 text-xs text-muted-foreground">Keyboard</p>
          <dl className="text-sm">
            {shortcuts.map(([does, key]) => (
              <div key={does} className="flex items-center justify-between px-2 py-1.5">
                <dt>{does}</dt>
                <dd className="text-xs tracking-widest text-muted-foreground">{key}</dd>
              </div>
            ))}
          </dl>
        </PopoverContent>
      </Popover>
    </footer>
  )
}
