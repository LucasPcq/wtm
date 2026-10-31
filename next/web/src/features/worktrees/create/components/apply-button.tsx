import { useQuery } from '@tanstack/react-query'
import { dryRunQuery } from '@/api/queries/commands'
import type { DomainAnswered } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { checkoutPlanned, planned } from '@/features/worktrees/create/utils/progress'
import type { Start } from '@/features/worktrees/hooks/use-run'

export function ApplyButton({
  repo,
  command,
  answers,
  onStart,
}: {
  repo: string
  command: 'create' | 'checkout'
  answers: DomainAnswered[]
  onStart: (start: Start) => void
}) {
  const dry = useQuery(dryRunQuery(repo, command, answers))
  const plan = dry.data?.plan
  const fingerprint = dry.data?.fingerprint

  const units = () => {
    if (!plan) return []
    return 'branches' in plan ? planned(plan) : checkoutPlanned(plan)
  }

  const label = () => {
    if (!plan) return 'Apply'
    if (!('creates' in plan)) return 'Create'
    if (plan.creates === 0) return 'Apply'
    return plan.creates === 1 ? 'Create' : `Create ${plan.creates} worktrees`
  }

  return (
    <Button
      disabled={!plan || !fingerprint}
      onClick={() => plan && fingerprint && onStart({ answers, fingerprint, units: units() })}
    >
      {label()}
    </Button>
  )
}
