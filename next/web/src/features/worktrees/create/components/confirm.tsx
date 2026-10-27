import { useQuery } from '@tanstack/react-query'
import { dryRunQuery } from '@/api/queries/commands'
import type { DomainAnswered } from '@/api/types.gen'
import { Skeleton } from '@/components/ui/skeleton'
import { Details } from '@/features/worktrees/components/details'
import { Facts } from '@/features/worktrees/components/facts'
import { ItemList } from '@/features/worktrees/components/item-list'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { BranchPlan } from '@/features/worktrees/create/components/branch-plan'
import { phaseLabel } from '@/features/worktrees/utils/phases'

export function Confirm({ repo, answers }: { repo: string; answers: DomainAnswered[] }) {
  const dry = useQuery(dryRunQuery(repo, 'create', answers))

  if (dry.isPending) return <Skeleton className="h-40 w-full" />
  if (dry.isError) return <p className="text-sm text-destructive">{dry.error.message}</p>

  const { plan, warnings } = dry.data
  const branches = plan?.branches ?? []
  const made = branches.filter((branch) => branch.action !== 'unchanged')
  const steps = made.flatMap((branch) => branch.phases ?? []).length
  const hooks = plan?.on_create ?? []
  const hooked = (branch: (typeof made)[number]) => hooks.length > 0 && (branch.phases ?? []).includes('on_create')
  const summary = [
    steps > 0 && `${steps} step${steps === 1 ? '' : 's'}`,
    made.some(hooked) && `on_create hook${hooks.length === 1 ? '' : 's'}`,
  ]
    .filter(Boolean)
    .join(', ')

  return (
    <div className="flex flex-col gap-6">
      <PlanWarnings warnings={warnings} />
      {plan && (
        <ItemList label="Branches">
          {branches.map((branch) => (
            <BranchPlan key={branch.branch} plan={plan} branch={branch} />
          ))}
        </ItemList>
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
                ...(hooked(branch) ? [{ label: 'on_create', values: hooks, mono: true }] : []),
              ]}
            />
          ))}
        </Details>
      )}
    </div>
  )
}
