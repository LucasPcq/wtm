import { CaseSensitive, ChevronDown, ChevronUp, CircleAlert, Clock, Ellipsis, Regex, Search } from 'lucide-react'
import { type RefObject, useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { InputGroup, InputGroupAddon, InputGroupInput } from '@/components/ui/input-group'
import { Popover, PopoverAnchor, PopoverContent } from '@/components/ui/popover'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Toggle } from '@/components/ui/toggle'
import type { LogDispatch, LogView } from '@/features/worktrees/running/hooks/use-log-view'

export function LogToolbar({
  view,
  dispatch,
  matches,
  position,
  onStep,
  invalid,
  search,
  onLatestError,
  onGoTo,
  timeOpen,
  onTimeOpen,
}: {
  view: LogView
  dispatch: LogDispatch
  matches: number
  position: number
  onStep: (by: 1 | -1) => void
  invalid: boolean
  search: RefObject<HTMLInputElement | null>
  onLatestError: () => void
  onGoTo: (time: string) => void
  timeOpen: boolean
  onTimeOpen: (open: boolean) => void
}) {
  const [time, setTime] = useState('')

  const counter = () => {
    if (view.query.text === '') return ''
    if (invalid) return 'not a regex'
    if (matches === 0) return 'no match'
    if (view.only) return `${matches} ${matches === 1 ? 'line' : 'lines'}`
    return `${position} of ${matches}`
  }

  return (
    <div role="toolbar" aria-label="Search this log" className="flex flex-wrap items-center gap-3 px-6 py-3">
      <InputGroup className="w-60">
        <InputGroupAddon>
          <Search />
        </InputGroupAddon>
        <InputGroupInput
          ref={search}
          aria-label="Search this log"
          placeholder="Search this log"
          value={view.query.text}
          onChange={(e) => dispatch({ type: 'search', change: { text: e.target.value } })}
          onKeyDown={(e) => {
            if (e.key === 'Enter') onStep(e.shiftKey ? -1 : 1)
            if (e.key === 'Escape') {
              dispatch({ type: 'search', change: { text: '' } })
              e.currentTarget.blur()
            }
          }}
        />
        <InputGroupAddon align="inline-end" className={invalid ? 'text-destructive' : 'tabular-nums'}>
          {counter()}
        </InputGroupAddon>
      </InputGroup>

      <div className="flex items-center gap-1.5">
        <Button
          variant="outline"
          size="icon"
          aria-label="Previous match"
          disabled={matches === 0}
          onClick={() => onStep(-1)}
        >
          <ChevronUp />
        </Button>
        <Button
          variant="outline"
          size="icon"
          aria-label="Next match"
          disabled={matches === 0}
          onClick={() => onStep(1)}
        >
          <ChevronDown />
        </Button>
        <Toggle
          variant="outline"
          aria-label="Match case"
          pressed={view.query.matchCase}
          onPressedChange={(matchCase) => dispatch({ type: 'search', change: { matchCase } })}
        >
          <CaseSensitive />
        </Toggle>
        <Toggle
          variant="outline"
          aria-label="Regex"
          pressed={view.query.regex}
          onPressedChange={(regex) => dispatch({ type: 'search', change: { regex } })}
        >
          <Regex />
        </Toggle>
      </div>

      <Tabs
        value={view.only ? 'only' : 'jump'}
        onValueChange={(mode) => dispatch({ type: 'only', only: mode === 'only' })}
      >
        <TabsList>
          <TabsTrigger value="jump">Jump</TabsTrigger>
          <TabsTrigger value="only">Only matching</TabsTrigger>
        </TabsList>
      </Tabs>

      <Popover open={timeOpen} onOpenChange={onTimeOpen}>
        <PopoverAnchor asChild>
          <div className="ml-auto flex items-center gap-1.5">
            {/* a narrow pane folds what is least used first: these two go into its menu */}
            <Button
              variant="outline"
              size="icon"
              aria-label="Latest error"
              className="@max-[760px]/panel:hidden"
              onClick={onLatestError}
            >
              <CircleAlert />
            </Button>
            <Button
              variant="outline"
              size="icon"
              aria-label="Go to time"
              className="@max-[760px]/panel:hidden"
              onClick={() => onTimeOpen(true)}
            >
              <Clock />
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline" size="icon" aria-label="More tools" className="@min-[760px]/panel:hidden">
                  <Ellipsis />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-48">
                <DropdownMenuItem onSelect={onLatestError}>
                  <CircleAlert />
                  Latest error
                  <DropdownMenuShortcut>E</DropdownMenuShortcut>
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={() => onTimeOpen(true)}>
                  <Clock />
                  Go to time
                  <DropdownMenuShortcut>T</DropdownMenuShortcut>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </PopoverAnchor>
        <PopoverContent align="end" className="w-60">
          <form
            className="flex items-center gap-2"
            onSubmit={(e) => {
              e.preventDefault()
              if (time === '') return
              onGoTo(time)
              onTimeOpen(false)
            }}
          >
            <Input type="time" step={1} aria-label="Time" value={time} onChange={(e) => setTime(e.target.value)} />
            <Button type="submit" size="sm">
              Go
            </Button>
          </form>
        </PopoverContent>
      </Popover>
    </div>
  )
}
