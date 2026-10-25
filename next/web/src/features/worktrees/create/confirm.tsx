import { useQuery } from '@tanstack/react-query'
import { dryRunQuery } from '@/api/queries/commands'
import type { CreateBranchPlan, CreatePlan, DomainAnswered } from '@/api/types.gen'
import { Skeleton } from '@/components/ui/skeleton'
import { phaseLabel } from './phases'
import { actionWords, envFrom, isolation } from './words'

// Confirm is what the apply will do, from the dry run's plan, not its recap, which is the command line's: a block
// per branch, where its worktree goes, what it starts from, its .env, its isolation and the steps it runs.
export function Confirm(props: { repo: string; answers: DomainAnswered[] }) {
  const dry = useQuery(dryRunQuery(props.repo, 'create', props.answers))
  if (dry.isPending) return <Skeleton className="h-40 w-full" />
  if (dry.isError) return <p className="text-sm text-destructive">{dry.error.message}</p>
  const plan = dry.data.plan
  const branches = plan?.branches ?? []
  const made = branches.filter((branch) => branch.action !== 'unchanged')
  return (
    <div className="flex flex-col gap-3">
      <h3 className="font-medium">
        {made.length === 1 ? `Create ${made[0]?.branch}` : `Create ${made.length} worktrees`}
      </h3>
      {plan && branches.map((branch) => <Branch key={branch.branch} plan={plan} branch={branch} />)}
      {dry.data.warnings.map((warning) => (
        <p key={warning.code} className="text-sm text-amber-600 dark:text-amber-500">
          {warning.message}
        </p>
      ))}
    </div>
  )
}

function Branch({ plan, branch }: { plan: CreatePlan; branch: CreateBranchPlan }) {
  const rows: { label: string; value: string; line?: string }[] =
    branch.action === 'unchanged'
      ? []
      : [
          { label: 'Starts from', value: source(plan, branch) },
          { label: '.env', value: envFrom[plan.env_from].title, line: envFrom[plan.env_from].description },
          ...(plan.isolation
            ? [
                {
                  label: 'Isolation',
                  value: isolation[plan.isolation].title,
                  line: isolation[plan.isolation].description,
                },
              ]
            : []),
          { label: 'Steps', value: (branch.phases ?? []).map((phase) => phaseLabel[phase]).join(' · ') },
        ]
  return (
    <section aria-label={branch.branch} className="flex flex-col gap-1 rounded-lg border p-3.5 text-sm">
      <div className="flex items-baseline justify-between gap-3">
        <span className="font-medium">{branch.branch}</span>
        <span className="text-xs text-muted-foreground">{actionWords[branch.action]}</span>
      </div>
      <p className="truncate text-left font-mono text-xs text-muted-foreground [direction:rtl]" title={branch.path}>
        <bdi>{branch.path}</bdi>
      </p>
      {rows.length > 0 && (
        <dl className="mt-2 grid grid-cols-[6.5rem_1fr] gap-x-4 gap-y-1.5">
          {rows.map((row) => (
            <div key={row.label} className="contents">
              <dt className="text-muted-foreground">{row.label}</dt>
              <dd className="text-pretty">
                {row.value}
                {row.line && <span className="block text-xs text-muted-foreground">{row.line}</span>}
              </dd>
            </div>
          ))}
        </dl>
      )}
    </section>
  )
}

// source is what a branch's worktree starts from: the plan's source for a new branch, fast-forwarded first when the
// plan says so, the branch itself or its remote otherwise.
function source(plan: CreatePlan, branch: CreateBranchPlan): string {
  switch (branch.action) {
    case 'create':
      return plan.from && plan.fast_forward?.includes(plan.from)
        ? `${plan.from}, fast-forwarded to origin first`
        : (plan.from ?? '')
    case 'check_out':
      return `${branch.branch}, as it is`
    case 'resume':
      return `origin/${branch.upstream ?? branch.branch}`
    case 'unchanged':
      return ''
  }
}
