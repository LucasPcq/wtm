import { useQuery } from '@tanstack/react-query'
import { dryRunQuery } from '@/api/queries/commands'
import type { DomainAnswered } from '@/api/types.gen'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert } from '@/features/worktrees/components/alert'
import { Details } from '@/features/worktrees/components/details'
import { Facts } from '@/features/worktrees/components/facts'
import { ItemList } from '@/features/worktrees/components/item-list'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { BranchPlan } from '@/features/worktrees/create/components/branch-plan'
import { planWide } from '@/features/worktrees/utils/outcome'
import { phaseLabel } from '@/features/worktrees/utils/phases'
import { refusalOf, sentence } from '@/features/worktrees/utils/sentence'
import { plural } from '@/lib/plural'

export function Confirm({ repo, answers }: { repo: string; answers: DomainAnswered[] }) {
  const dry = useQuery(dryRunQuery(repo, 'create', answers))

  if (dry.isPending) return <Skeleton className="h-40 w-full" />
  if (dry.isError) return <Alert tone="red" description={sentence(dry.error.message)} />
  if (dry.data.error) return <Alert tone="red" description={refusalOf(dry.data.error)} />

  const { plan, warnings } = dry.data
  const branches = plan?.branches ?? []
  // a warning about a branch goes on the row of that branch, or of the branches created from it
  const warningsOf = (branch: (typeof branches)[number]) =>
    warnings.filter(
      (warning) =>
        !planWide(warning) &&
        (warning.params?.branch === branch.branch ||
          (branch.action === 'create' && warning.params?.branch === plan?.from)),
    )
  const alerts = warnings.filter((warning) => !branches.some((branch) => warningsOf(branch).includes(warning)))
  // the parent strategy with no source worktree copies main's .env, as the plan's Alert says
  const fellBack = warnings.some((warning) => warning.code === 'create.env_parent_fallback')
  const made = branches.filter((branch) => branch.action !== 'unchanged')
  const steps = made.flatMap((branch) => branch.phases ?? []).length
  const hooks = plan?.on_create ?? []
  const hooked = (branch: (typeof made)[number]) => hooks.length > 0 && (branch.phases ?? []).includes('on_create')
  const summary = [
    steps > 0 && plural(steps, 'step'),
    made.some(hooked) && `on_create hook${hooks.length === 1 ? '' : 's'}`,
  ]
    .filter(Boolean)
    .join(', ')

  return (
    <div className="flex flex-col gap-6">
      <PlanWarnings warnings={alerts} />
      {plan && (
        <ItemList label="Branches">
          {branches.map((branch) => (
            <BranchPlan
              key={branch.branch}
              plan={plan}
              branch={branch}
              envFrom={fellBack ? 'main' : plan.env_from}
              warnings={warningsOf(branch)}
            />
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
