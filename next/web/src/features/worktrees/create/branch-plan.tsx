import type { CreateBranchPlan, CreatePlan } from '@/api/types.gen'
import { actionWords, envFrom, isolation, phaseLabel } from '@/features/worktrees/create/words'
import { Path } from '@/features/worktrees/shared/path'

type Fact = { label: string; value: string; line?: string }

export function BranchPlan({ plan, branch }: { plan: CreatePlan; branch: CreateBranchPlan }) {
  const source = () => {
    switch (branch.action) {
      case 'create':
        if (plan.from && plan.fast_forward?.includes(plan.from)) return `${plan.from}, fast-forwarded to origin first`
        return plan.from ?? ''
      case 'check_out':
        return `${branch.branch}, as it is`
      case 'resume':
        return `origin/${branch.upstream ?? branch.branch}`
      case 'unchanged':
        return ''
    }
  }

  const facts = (): Fact[] => {
    if (branch.action === 'unchanged') return []

    const env = envFrom[plan.env_from]
    const facts: Fact[] = [
      { label: 'Starts from', value: source() },
      { label: '.env', value: env.title, line: env.description },
    ]
    if (plan.isolation) {
      const chosen = isolation[plan.isolation]
      facts.push({ label: 'Isolation', value: chosen.title, line: chosen.description })
    }
    facts.push({ label: 'Steps', value: (branch.phases ?? []).map((phase) => phaseLabel[phase]).join(' · ') })
    return facts
  }

  return (
    <section aria-label={branch.branch} className="flex flex-col gap-1 rounded-lg border p-3.5 text-sm">
      <div className="flex items-baseline justify-between gap-3">
        <span className="font-medium">{branch.branch}</span>
        <span className="text-xs text-muted-foreground">{actionWords[branch.action]}</span>
      </div>
      <Path path={branch.path} />

      {facts().length > 0 && (
        <dl className="mt-2 grid grid-cols-[6.5rem_1fr] gap-x-4 gap-y-1.5">
          {facts().map(({ label, value, line }) => (
            <div key={label} className="contents">
              <dt className="text-muted-foreground">{label}</dt>
              <dd className="text-pretty">
                {value}
                {line && <span className="block text-xs text-muted-foreground">{line}</span>}
              </dd>
            </div>
          ))}
        </dl>
      )}
    </section>
  )
}
