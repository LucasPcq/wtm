import { useState } from 'react'
import type { DomainOption } from '@/api/types.gen'
import { Checkbox } from '@/components/ui/checkbox'
import { Item, ItemContent, ItemDescription, ItemTitle } from '@/components/ui/item'
import { ItemList } from '@/features/worktrees/components/item-list'
import { type Left, LeftItem } from '@/features/worktrees/components/left-item'
import { ListHeader } from '@/features/worktrees/components/list-header'
import { parentWords } from '@/features/worktrees/reparent/utils/words'
import { searched } from '@/features/worktrees/utils/filters'

export function WorktreesStep({
  options,
  checked,
  left,
  onCheck,
}: {
  options: DomainOption[]
  checked: string[]
  left: Left[]
  onCheck: (worktrees: string[], on: boolean) => void
}) {
  const [query, setQuery] = useState('')
  const visible = options.filter((option) => searched(option.value, query)).map((option) => option.value)

  return (
    <ItemList
      label="Worktrees"
      header={
        <ListHeader
          total={options.length}
          visible={visible}
          bulk={visible}
          checked={checked}
          query={query}
          onQuery={setQuery}
          onCheck={onCheck}
        />
      }
      left={left.map(({ worktree, why }) => <LeftItem key={worktree} worktree={worktree} why={why} />)}
    >
      {options
        .filter((option) => visible.includes(option.value))
        .map(({ value, parent = '', gone = false }) => (
          <Item key={value} role="listitem" aria-label={value} className="items-center gap-3 rounded-none px-4 py-3">
            <Checkbox
              aria-label={`Reparent ${value}`}
              checked={checked.includes(value)}
              onCheckedChange={(on) => onCheck([value], on === true)}
            />
            <ItemContent className="gap-1">
              <ItemTitle className="min-w-0">{value}</ItemTitle>
              <ItemDescription className="text-xs">
                {parent ? `on ${parentWords({ from: parent, parent_gone: gone })}` : 'no parent'}
              </ItemDescription>
            </ItemContent>
          </Item>
        ))}
    </ItemList>
  )
}
