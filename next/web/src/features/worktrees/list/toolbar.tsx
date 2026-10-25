import { useDebouncedCallback } from '@tanstack/react-pacer'
import { useNavigate } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { InputGroup, InputGroupAddon, InputGroupInput } from '@/components/ui/input-group'
import { Kbd } from '@/components/ui/kbd'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import type { WorktreesSearch } from '../search'
import { filterLabel, filters, type Filter } from './filters'

// focusOnSlash focuses the search when / is typed outside a field, as the hint beside it says.
function focusOnSlash(input: HTMLInputElement | null) {
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

// Toolbar is the view, the search and the filters of the screen, all in its address; trailing is what a view adds
// at the end of the row (the columns of the list).
export function Toolbar(props: {
  search: WorktreesSearch
  counts?: Record<Filter, number | undefined>
  trailing?: ReactNode
}) {
  const navigate = useNavigate({ from: '/r/$repo/worktrees' })
  const set = (change: Partial<WorktreesSearch>) =>
    void navigate({ search: (search) => ({ ...search, ...change }), replace: true })
  const filter = props.search.filter ?? 'all'
  // the search goes to the address once typing pauses; a search cleared from elsewhere (No match) empties the box
  const [query, setQuery] = useState(props.search.q ?? '')
  const [asked, setAsked] = useState(props.search.q)
  if (props.search.q !== asked) {
    setAsked(props.search.q)
    if (props.search.q === undefined) setQuery('')
  }
  const search = useDebouncedCallback((q: string) => set({ q: q.trim() || undefined }), { wait: 250 })

  return (
    <div className="flex flex-wrap items-center gap-3 border-b px-8 py-3">
      <Tabs
        value={props.search.view ?? 'list'}
        onValueChange={(view) => set({ view: view === 'tree' ? 'tree' : undefined })}
      >
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
          value={query}
          onChange={(e) => {
            setQuery(e.target.value)
            search(e.target.value)
          }}
        />
        <InputGroupAddon align="inline-end">
          <Kbd>/</Kbd>
        </InputGroupAddon>
      </InputGroup>
      {props.counts && (
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          aria-label="Filter"
          value={filter}
          onValueChange={(value: string) => {
            const next = filters.find((known) => known === value)
            if (next) set({ filter: next === 'all' ? undefined : next })
          }}
        >
          {filters.map((known) => (
            <ToggleGroupItem key={known} value={known} className="rounded-full px-3">
              {filterLabel[known]}
              <span className="text-xs text-muted-foreground tabular-nums">{props.counts?.[known] ?? '—'}</span>
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      )}
      <div className="ml-auto">{props.trailing}</div>
    </div>
  )
}
