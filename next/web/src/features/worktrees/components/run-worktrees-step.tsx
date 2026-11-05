import { useState } from 'react'
import type { DomainOption } from '@/api/types.gen'
import { Checkbox } from '@/components/ui/checkbox'
import { Item, ItemContent, ItemDescription, ItemTitle } from '@/components/ui/item'
import { ItemList } from '@/features/worktrees/components/item-list'
import { ListHeader } from '@/features/worktrees/components/list-header'
import { searched } from '@/features/worktrees/utils/filters'
import { runsNow } from '@/features/worktrees/utils/run-words'

// a run's worktrees step, Start a profile's and Stop jobs': each row says what the worktree runs now
export function RunWorktreesStep({
  options,
  checked,
  verb,
  note,
  onCheck,
}: {
  options: DomainOption[]
  checked: string[]
  verb: string
  note?: string
  onCheck: (worktrees: string[], on: boolean) => void
}) {
  const [query, setQuery] = useState('')
  const visible = options.filter((option) => searched(option.value, query))
  const names = visible.map((option) => option.value)

  return (
    <ItemList
      label="Worktrees"
      header={
        <ListHeader
          total={options.length}
          visible={names}
          bulk={names}
          checked={checked}
          query={query}
          note={note}
          onQuery={setQuery}
          onCheck={onCheck}
        />
      }
    >
      {visible.map((option) => (
        <Item
          key={option.value}
          role="listitem"
          aria-label={option.value}
          className="items-start gap-3 rounded-none px-4 py-3"
        >
          <Checkbox
            aria-label={`${verb} ${option.value}`}
            className="mt-0.5"
            checked={checked.includes(option.value)}
            onCheckedChange={(on) => onCheck([option.value], on === true)}
          />
          <ItemContent className="gap-1">
            <ItemTitle className="min-w-0">{option.value}</ItemTitle>
            <ItemDescription className="text-xs">{runsNow(option)}</ItemDescription>
          </ItemContent>
        </Item>
      ))}
    </ItemList>
  )
}
