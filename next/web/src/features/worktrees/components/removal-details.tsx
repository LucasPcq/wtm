import type { TeardownRemoval } from '@/api/types.gen'
import { Details } from '@/features/worktrees/components/details'
import { type Fact, Facts } from '@/features/worktrees/components/facts'

export function RemovalDetails({ removals }: { removals: TeardownRemoval[] }) {
  const hooks = removals.flatMap((removal) => removal.hooks ?? []).length
  const keeping = removals.filter((removal) => (removal.namespaces ?? []).length > 0).length
  const summary = [
    hooks > 0 && (hooks === 1 ? '1 on_clean hook runs first' : `${hooks} on_clean hooks run first`),
    keeping > 0 && `data stays for ${keeping}`,
  ]
    .filter(Boolean)
    .join(', ')

  const factsOf = (removal: TeardownRemoval): Fact[] => [
    ...(removal.path ? [{ label: 'Folder', values: [removal.path], mono: true }] : []),
    ...((removal.hooks ?? []).length > 0 ? [{ label: 'on_clean', values: removal.hooks ?? [], mono: true }] : []),
    ...((removal.namespaces ?? []).length > 0
      ? [
          {
            label: 'Data',
            values: [`${removal.namespaces?.join(', ')} stays in its services`],
            line: 'wtm-next does not drop it yet',
          },
        ]
      : []),
  ]

  return (
    <Details label="Details" summary={summary}>
      {removals.map((removal) => (
        <Facts key={removal.worktree} heading={removal.worktree} facts={factsOf(removal)} />
      ))}
    </Details>
  )
}
