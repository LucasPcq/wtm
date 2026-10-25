import { useDebouncer } from '@tanstack/react-pacer'
import { useNavigate } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { InputGroup, InputGroupAddon, InputGroupInput } from '@/components/ui/input-group'
import { Kbd } from '@/components/ui/kbd'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { type Filter, filterLabel, filters } from '@/features/worktrees/list/utils/filters'
import type { WorktreesSearch } from '@/features/worktrees/utils/search'

export function Toolbar({
  search,
  counts,
  trailing,
}: {
  search: WorktreesSearch
  counts?: Record<Filter, number | undefined>
  trailing?: ReactNode
}) {
  const navigate = useNavigate({ from: '/r/$repo/worktrees' })
  const set = (change: Partial<WorktreesSearch>) =>
    void navigate({ search: (current) => ({ ...current, ...change }), replace: true })
  const ask = useDebouncer((q: string) => set({ q: q.trim() || undefined }), { wait: 250 })

  // the box goes to the address once typing pauses; a search cleared from elsewhere (No match) empties it, and
  // drops what was still to be sent, which would bring it back
  const [box, setBox] = useState({ typed: search.q ?? '', asked: search.q })
  if (search.q !== box.asked) {
    setBox({ typed: search.q === undefined ? '' : box.typed, asked: search.q })
    if (search.q === undefined) ask.cancel()
  }

  // / focuses the search from anywhere but a field, as the hint beside it says
  const focusOnSlash = (input: HTMLInputElement | null) => {
    if (!input) return

    const onKey = (e: KeyboardEvent) => {
      const typing = e.target instanceof HTMLElement && e.target.closest('input, textarea, select, [contenteditable]')
      if (e.key !== '/' || typing || e.metaKey || e.ctrlKey || e.altKey) return
      e.preventDefault()
      input.focus()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }

  return (
    <div className="flex flex-wrap items-center gap-3 border-b px-8 py-3">
      <Tabs value={search.view ?? 'list'} onValueChange={(view) => set({ view: view === 'tree' ? 'tree' : undefined })}>
        <TabsList>
          <TabsTrigger value="list">List</TabsTrigger>
          <TabsTrigger value="tree">Tree</TabsTrigger>
        </TabsList>
      </Tabs>

      <InputGroup className="w-60">
        <InputGroupAddon>
          <Search />
        </InputGroupAddon>
        <InputGroupInput
          ref={focusOnSlash}
          aria-label="Search branches"
          placeholder="Search branches"
          value={box.typed}
          onChange={(e) => {
            setBox({ ...box, typed: e.target.value })
            ask.maybeExecute(e.target.value)
          }}
        />
        <InputGroupAddon align="inline-end">
          <Kbd>/</Kbd>
        </InputGroupAddon>
      </InputGroup>

      {counts && (
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          aria-label="Filter"
          value={search.filter ?? 'all'}
          onValueChange={(value: string) => {
            const next = filters.find((known) => known === value)
            if (next) set({ filter: next === 'all' ? undefined : next })
          }}
        >
          {filters.map((known) => (
            <ToggleGroupItem key={known} value={known} className="rounded-full px-3">
              {filterLabel[known]}
              <span className="text-xs text-muted-foreground tabular-nums">{counts[known] ?? '—'}</span>
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      )}

      <div className="ml-auto">{trailing}</div>
    </div>
  )
}
