import { Search } from 'lucide-react'
import { InputGroup, InputGroupAddon, InputGroupInput } from '@/components/ui/input-group'
import { TriCheckbox } from '@/features/worktrees/components/tri-checkbox'

// the box never checks an unsafe row (bulk): one click must not force it; a checked row the search hides stays
// checked, and the count says so
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
