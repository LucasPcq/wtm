import { useQuery } from '@tanstack/react-query'
import { dryRunQuery, reviewQuery } from '@/api/queries/commands'
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
  const created = useQuery({ ...dryRunQuery(repo, 'create', answers), enabled: command === 'create' })
  // checkout's plan fetches the pull request's branch: asked once per opening, as Confirm asks it
  const checkedOut = useQuery({ ...reviewQuery(repo, 'checkout', answers), enabled: command === 'checkout' })

  const ready = () => {
    if (command === 'checkout') {
      const plan = checkedOut.data?.plan
      return plan && { label: 'Create', fingerprint: checkedOut.data?.fingerprint, units: checkoutPlanned(plan) }
    }
    const plan = created.data?.plan
    if (!plan) return undefined
    const label = plan.creates === 1 ? 'Create' : `Create ${plan.creates} worktrees`
    return { label: plan.creates === 0 ? 'Apply' : label, fingerprint: created.data?.fingerprint, units: planned(plan) }
  }
  const plan = ready()

  return (
    <Button
      disabled={!plan?.fingerprint}
      onClick={() => plan?.fingerprint && onStart({ answers, fingerprint: plan.fingerprint, units: plan.units })}
    >
      {plan?.label ?? 'Apply'}
    </Button>
  )
}
