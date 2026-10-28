import { useState } from 'react'
import { Checkbox } from '@/components/ui/checkbox'
import { Item, ItemTitle } from '@/components/ui/item'
import { ItemList } from '@/features/worktrees/components/item-list'
import { type Left, LeftItem } from '@/features/worktrees/components/left-item'
import { ListHeader } from '@/features/worktrees/components/list-header'
import { searched } from '@/features/worktrees/utils/filters'

export function WorktreesStep({
  options,
  checked,
  left,
  onCheck,
}: {
  options: string[]
  checked: string[]
  left: Left[]
  onCheck: (worktrees: string[], on: boolean) => void
}) {
  const [query, setQuery] = useState('')
  const visible = options.filter((worktree) => searched(worktree, query))

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
      {visible.map((worktree) => (
        <Item
          key={worktree}
          role="listitem"
          aria-label={worktree}
          className="items-center gap-3 rounded-none px-4 py-3"
        >
          <Checkbox
            aria-label={`Run in ${worktree}`}
            checked={checked.includes(worktree)}
            onCheckedChange={(on) => onCheck([worktree], on === true)}
          />
          <ItemTitle className="min-w-0">{worktree}</ItemTitle>
        </Item>
      ))}
    </ItemList>
  )
}
