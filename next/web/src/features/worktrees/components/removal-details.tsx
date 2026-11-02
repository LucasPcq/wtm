import type { TeardownRemoval } from '@/api/types.gen'
import { Details } from '@/features/worktrees/components/details'
import { type Fact, Facts } from '@/features/worktrees/components/facts'
import { dataWords } from '@/features/worktrees/utils/data-words'

export function RemovalDetails({ removals }: { removals: TeardownRemoval[] }) {
  const hooks = removals.flatMap((removal) => removal.hooks ?? []).length
  const holding = removals.filter((removal) => (removal.data ?? []).length > 0).length
  const summary = [
    hooks > 0 && (hooks === 1 ? '1 on_clean hook runs first' : `${hooks} on_clean hooks run first`),
    holding > 0 && `data in shared services for ${holding}`,
  ]
    .filter(Boolean)
    .join(', ')

  const factsOf = (removal: TeardownRemoval): Fact[] => [
    ...(removal.path ? [{ label: 'Folder', values: [removal.path], mono: true }] : []),
    ...((removal.hooks ?? []).length > 0 ? [{ label: 'on_clean', values: removal.hooks ?? [], mono: true }] : []),
    ...((removal.data ?? []).length > 0
      ? [
          {
            label: 'Data',
            values: (removal.data ?? []).map(
              (namespace) => `${namespace.name} in ${namespace.job} ${dataWords[namespace.status](namespace)}`,
            ),
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
