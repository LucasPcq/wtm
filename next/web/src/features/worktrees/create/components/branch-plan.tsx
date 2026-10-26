import type { CreateBranchPlan, CreatePlan } from '@/api/types.gen'
import { Item, ItemTitle } from '@/components/ui/item'
import { actionWords, envFromShort, isolationShort } from '@/features/worktrees/create/utils/words'

export function BranchPlan({ plan, branch }: { plan: CreatePlan; branch: CreateBranchPlan }) {
  const source = () => {
    switch (branch.action) {
      case 'create':
        if (!plan.from) return undefined
        return plan.fast_forward?.includes(plan.from)
          ? `From ${plan.from}, updated from origin first`
          : `From ${plan.from}`
      case 'resume':
        return `From origin/${branch.upstream ?? branch.branch}`
      case 'check_out':
      case 'unchanged':
        return undefined
    }
  }

  const line =
    branch.action === 'unchanged'
      ? []
      : [source(), plan.isolation && isolationShort[plan.isolation], envFromShort[plan.env_from]].filter(Boolean)

  return (
    <Item role="listitem" aria-label={branch.branch} className="flex-col items-stretch gap-1 rounded-none px-4 py-3">
      <div className="flex items-baseline gap-3">
        <ItemTitle className="min-w-0 flex-1">{branch.branch}</ItemTitle>
        <span className="text-xs text-muted-foreground">{actionWords[branch.action]}</span>
      </div>
      {line.length > 0 && <p className="text-xs text-muted-foreground">{line.join(' · ')}</p>}
    </Item>
  )
}
