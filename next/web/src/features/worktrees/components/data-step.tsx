import type { TeardownRemoval } from '@/api/types.gen'
import { ChoiceOptions } from '@/components/choice-options'
import { Item } from '@/components/ui/item'
import { ItemList } from '@/features/worktrees/components/item-list'
import { listed } from '@/features/worktrees/utils/listed'
import { downOf } from '@/features/worktrees/utils/removal'

export function DataStep({
  removals,
  dropData,
  onDropData,
}: {
  removals: TeardownRemoval[]
  dropData: boolean
  onDropData: (on: boolean) => void
}) {
  const down = downOf(removals)
  const services = listed(down)
  const held = removals.flatMap((removal) =>
    (removal.data ?? [])
      .filter((namespace) => down.includes(namespace.job))
      .map((namespace) => ({ worktree: removal.worktree, namespace })),
  )

  return (
    <>
      <p className="text-sm">{`${services} ${down.length === 1 ? 'is' : 'are'} down: the data these worktrees made there can't be dropped yet.`}</p>

      <ItemList label="Data held">
        {held.map(({ worktree, namespace }) => (
          <Item
            key={`${worktree}-${namespace.job}-${namespace.name}`}
            role="listitem"
            aria-label={worktree}
            className="gap-3 rounded-none px-4 py-2.5"
          >
            <span className="w-44 shrink-0 truncate font-medium">{worktree}</span>
            <span className="min-w-0 truncate text-muted-foreground">{`${namespace.name} in ${namespace.job}`}</span>
          </Item>
        ))}
      </ItemList>

      <ChoiceOptions
        id="drop-data"
        label="What becomes of their data"
        value={dropData ? 'start' : 'defer'}
        options={[
          {
            value: 'defer',
            title: `Keep it until ${services} next starts`,
            description: 'The next run up or prune drops it.',
          },
          {
            value: 'start',
            title: `Start ${services} and drop it now`,
            description: 'Stopped again once the data is dropped.',
          },
        ]}
        onChange={(value) => onDropData(value === 'start')}
      />
    </>
  )
}
