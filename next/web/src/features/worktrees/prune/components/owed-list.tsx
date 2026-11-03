import type { TeardownDebt } from '@/api/types.gen'
import { Item } from '@/components/ui/item'
import { ItemList } from '@/features/worktrees/components/item-list'

export function OwedList({ owed }: { owed: TeardownDebt[] }) {
  return (
    <section aria-label="Owed" className="flex flex-col gap-2">
      <h3 className="text-xs font-medium text-muted-foreground">Owed by worktrees removed earlier, dropped first</h3>
      <ItemList>
        {owed.map(({ job, name, worktree }) => (
          <Item key={`${job}-${name}`} role="listitem" aria-label={name} className="gap-3 rounded-none px-4 py-2.5">
            <span className="w-44 shrink-0 truncate font-medium">{name}</span>
            <span className="truncate text-muted-foreground">{`in ${job} · owed since ${worktree} was removed`}</span>
          </Item>
        ))}
      </ItemList>
    </section>
  )
}
