import { Search } from 'lucide-react'
import { InputGroup, InputGroupAddon, InputGroupInput } from '@/components/ui/input-group'
import { TriCheckbox } from '@/features/worktrees/components/tri-checkbox'

// from this many rows a search sits above them: below it, the whole list fits and reading beats searching
export const searchFrom = 8

// the box acts on the rows the search shows: it checks only those it may check at once (bulk: never an unsafe one,
// which one click must not force), and clears them all, clearing being the safe way; a checked row the search hides
// stays checked, and the count says so
export function ListHeader({
  total,
  visible,
  bulk,
  checked,
  query,
  onQuery,
  onCheck,
}: {
  total: number
  visible: string[]
  bulk: string[]
  checked: string[]
  query: string
  onQuery: (query: string) => void
  onCheck: (worktrees: string[], on: boolean) => void
}) {
  const hidden = checked.filter((worktree) => !visible.includes(worktree)).length
  const shownChecked = visible.filter((worktree) => checked.includes(worktree)).length
  const bulkChecked = bulk.filter((worktree) => checked.includes(worktree)).length
  const all = () => {
    if (shownChecked === 0) return false
    return bulkChecked === bulk.length ? true : 'indeterminate'
  }

  return (
    <div className="flex flex-col">
      {total >= searchFrom && (
        <InputGroup className="rounded-none border-0 border-b shadow-none has-[[data-slot=input-group-control]:focus-visible]:ring-0">
          <InputGroupAddon>
            <Search />
          </InputGroupAddon>
          <InputGroupInput
            aria-label="Search the worktrees"
            placeholder="Search worktrees"
            value={query}
            onChange={(event) => onQuery(event.target.value)}
          />
        </InputGroup>
      )}
      <div className="flex items-center gap-3 px-4 py-2.5 text-xs text-muted-foreground">
        <TriCheckbox
          label="Select every worktree shown"
          checked={all()}
          disabled={bulk.length === 0 && shownChecked === 0}
          onToggle={() => (all() === true ? onCheck(visible, false) : onCheck(bulk, true))}
        />
        {checked.length} of {total} selected
        {hidden > 0 && ` · ${hidden} hidden by the search`}
      </div>
    </div>
  )
}
