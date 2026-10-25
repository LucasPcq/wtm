import { useQuery } from '@tanstack/react-query'
import { dryRunQuery } from '@/api/queries/commands'
import type { DomainAnswered } from '@/api/types.gen'
import { Skeleton } from '@/components/ui/skeleton'
import { BranchPlan } from '@/features/worktrees/create/branch-plan'

export function Confirm({ repo, answers }: { repo: string; answers: DomainAnswered[] }) {
  const dry = useQuery(dryRunQuery(repo, 'create', answers))

  if (dry.isPending) return <Skeleton className="h-40 w-full" />
  if (dry.isError) return <p className="text-sm text-destructive">{dry.error.message}</p>

  const { plan, warnings } = dry.data
  const branches = plan?.branches ?? []
  const made = branches.filter((branch) => branch.action !== 'unchanged')

  return (
    <div className="flex flex-col gap-3">
      <h3 className="font-medium">
        {made.length === 1 ? `Create ${made[0]?.branch}` : `Create ${made.length} worktrees`}
      </h3>
      {plan && branches.map((branch) => <BranchPlan key={branch.branch} plan={plan} branch={branch} />)}
      {warnings.map((warning) => (
        <p key={warning.code} className="text-sm text-amber-600 dark:text-amber-500">
          {warning.message}
        </p>
      ))}
    </div>
  )
}
