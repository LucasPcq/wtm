import { useQuery } from '@tanstack/react-query'
import { TriangleAlert } from 'lucide-react'
import { dryRunQuery } from '@/api/queries/commands'
import type { DomainAnswered } from '@/api/types.gen'
import { Skeleton } from '@/components/ui/skeleton'
import { Details } from '@/features/worktrees/components/details'
import { Facts } from '@/features/worktrees/components/facts'
import { ItemList } from '@/features/worktrees/components/item-list'
import { BranchPlan } from '@/features/worktrees/create/components/branch-plan'
import { phaseLabel } from '@/features/worktrees/utils/phases'
import { sentence } from '@/features/worktrees/utils/sentence'

export function Confirm({ repo, answers }: { repo: string; answers: DomainAnswered[] }) {
  const dry = useQuery(dryRunQuery(repo, 'create', answers))

  if (dry.isPending) return <Skeleton className="h-40 w-full" />
  if (dry.isError) return <p className="text-sm text-destructive">{dry.error.message}</p>

  const { plan, warnings } = dry.data
  const branches = plan?.branches ?? []
  const made = branches.filter((branch) => branch.action !== 'unchanged')
  const steps = made.flatMap((branch) => branch.phases ?? []).length
  const hooks = plan?.on_create ?? []
  const summary = [
    steps > 0 && `${steps} step${steps === 1 ? '' : 's'}`,
    hooks.length > 0 && `on_create hook${hooks.length === 1 ? '' : 's'}`,
  ]
    .filter(Boolean)
    .join(', ')

  return (
    <div className="flex flex-col gap-6">
      {plan && (
        <ItemList label="Branches">
          {branches.map((branch) => (
            <BranchPlan key={branch.branch} plan={plan} branch={branch} />
          ))}
        </ItemList>
      )}
      {warnings.length > 0 && (
        <ul className="flex flex-col gap-2 text-sm">
          {warnings.map((warning) => (
            <li key={warning.code} className="flex items-start gap-3">
              <TriangleAlert className="mt-0.5 size-4 shrink-0 text-amber-500" aria-hidden="true" />
              {sentence(warning.message)}
            </li>
          ))}
        </ul>
      )}
      {made.length > 0 && (
        <Details label="Details" summary={summary}>
          {made.map((branch) => (
            <Facts
              key={branch.branch}
              heading={made.length > 1 ? branch.branch : undefined}
              facts={[
                { label: 'Folder', values: [branch.path], mono: true },
                {
                  label: 'Steps',
                  values: [
                    (branch.phases ?? [])
                      .map((phase, i) => (i === 0 ? phaseLabel[phase] : phaseLabel[phase].toLowerCase()))
                      .join(', '),
                  ],
                },
                ...(hooks.length > 0 ? [{ label: 'on_create', values: hooks, mono: true }] : []),
              ]}
            />
          ))}
        </Details>
      )}
    </div>
  )
}
