import { GitBranch, ListFilter, Search } from 'lucide-react'
import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { InputGroup, InputGroupAddon, InputGroupInput } from '@/components/ui/input-group'
import { Kbd } from '@/components/ui/kbd'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { chipLabel, shownLabel } from '@/features/worktrees/running/utils/list-words'
import type { Chip, RunFilter, ShownState } from '@/features/worktrees/store/ui-store'
import { focusOnSlash } from '@/features/worktrees/utils/slash'
import { cn } from '@/lib/utils'

const chips: Chip[] = ['all', 'running', 'crashed']
const states: ShownState[] = ['running', 'crashed', 'ended']
const all = 'all'

// the list's filter: the search, the chips with their counts, what the filter hides and its Clear; the Filter menu
// picks a worktree in a submenu, which a repository with many keeps out of the way, and the states the list shows
export function RunningToolbar({
  search,
  onSearch,
  filter,
  onFilter,
  onClear,
  counts,
  stateCounts,
  hidden,
  worktrees,
  switcher,
}: {
  search: string
  onSearch: (search: string) => void
  filter: RunFilter
  onFilter: (change: Partial<RunFilter>) => void
  onClear: () => void
  counts: Record<Chip, number>
  stateCounts: Record<ShownState, number>
  hidden: number
  worktrees: { key: string; shared: boolean; label: string }[]
  switcher: ReactNode
}) {
  // a window too narrow for the list beside a job's log folds the list into the switcher, beside Crashed and Filter
  const folds = switcher ? '@max-[1100px]/page:hidden' : undefined
  const shared = worktrees.find((one) => one.shared)
  const own = worktrees.filter((one) => !one.shared)
  const chosen = () => {
    if (filter.worktree === undefined) return 'All worktrees'
    if (filter.worktree === shared?.key) return 'Shared services'
    // a worktree whose jobs all ended is no group any more: its key still names its folder
    return worktrees.find((one) => one.key === filter.worktree)?.label ?? filter.worktree.split('/').pop()
  }
  return (
    <div className="flex flex-wrap items-center gap-3 border-b px-8 py-3">
      {switcher}
      <InputGroup className={cn('w-60', folds)}>
        <InputGroupAddon>
          <Search />
        </InputGroupAddon>
        <InputGroupInput
          ref={focusOnSlash}
          aria-label="Filter jobs"
          placeholder="Filter jobs"
          value={search}
          onChange={(e) => onSearch(e.target.value)}
        />
        <InputGroupAddon align="inline-end">
          <Kbd>/</Kbd>
        </InputGroupAddon>
      </InputGroup>

      <ToggleGroup
        type="single"
        variant="outline"
        size="sm"
        aria-label="State"
        value={filter.chip}
        onValueChange={(value: string) => {
          const chip = chips.find((known) => known === value)
          if (chip) onFilter({ chip })
        }}
      >
        {chips.map((chip) => (
          <ToggleGroupItem key={chip} value={chip} className={cn('rounded-full px-3', chip !== 'crashed' && folds)}>
            {chipLabel[chip]}
            <span className="text-xs text-muted-foreground tabular-nums">{counts[chip]}</span>
          </ToggleGroupItem>
        ))}
      </ToggleGroup>

      {hidden > 0 && (
        <span className="flex items-center gap-1 text-sm text-muted-foreground">
          {hidden} hidden by the filter
          <Button variant="ghost" size="sm" onClick={onClear}>
            Clear
          </Button>
        </span>
      )}

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="sm" className="ml-auto">
            <ListFilter />
            Filter
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-72">
          <DropdownMenuSub>
            <DropdownMenuSubTrigger>
              <GitBranch className="text-muted-foreground" aria-hidden="true" />
              <span className="flex-1">Worktree</span>
              <span className="max-w-32 truncate text-muted-foreground">{chosen()}</span>
            </DropdownMenuSubTrigger>
            <DropdownMenuSubContent className="max-h-80 w-56 overflow-y-auto">
              <DropdownMenuRadioGroup
                value={filter.worktree ?? all}
                onValueChange={(value) => onFilter({ worktree: value === all ? undefined : value })}
              >
                <DropdownMenuRadioItem value={all}>All worktrees</DropdownMenuRadioItem>
                {shared && <DropdownMenuRadioItem value={shared.key}>Shared services</DropdownMenuRadioItem>}
                {own.length > 0 && <DropdownMenuSeparator />}
                {own.map(({ key, label }) => (
                  <DropdownMenuRadioItem key={key} value={key}>
                    {label}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuSubContent>
          </DropdownMenuSub>
          <DropdownMenuSeparator />
          <DropdownMenuLabel className="text-xs text-muted-foreground">State</DropdownMenuLabel>
          {states.map((state) => (
            <DropdownMenuCheckboxItem
              key={state}
              checked={filter.states.includes(state)}
              onSelect={(e) => e.preventDefault()}
              onCheckedChange={(checked) =>
                onFilter({ states: checked ? [...filter.states, state] : filter.states.filter((one) => one !== state) })
              }
            >
              <span className="flex-1">{shownLabel[state]}</span>
              <span className="text-xs text-muted-foreground tabular-nums">{stateCounts[state]}</span>
            </DropdownMenuCheckboxItem>
          ))}
          <DropdownMenuSeparator />
          <p className="px-2 py-1.5 text-xs text-muted-foreground">Kept for you in this browser</p>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
