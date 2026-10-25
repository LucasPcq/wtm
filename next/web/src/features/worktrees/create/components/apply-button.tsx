import { useQuery } from '@tanstack/react-query'
import { dryRunQuery } from '@/api/queries/commands'
import type { DomainAnswered } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import type { Planned } from '@/features/worktrees/create/utils/progress'

export function ApplyButton({
  repo,
  answers,
  onStart,
}: {
  repo: string
  answers: DomainAnswered[]
  onStart: (start: Planned) => void
}) {
  const dry = useQuery(dryRunQuery(repo, 'create', answers))
  const plan = dry.data?.plan
  const fingerprint = dry.data?.fingerprint

  const label = () => {
    if (!plan || plan.creates === 0) return 'Apply'
    return plan.creates === 1 ? 'Create worktree' : `Create ${plan.creates} worktrees`
  }

  return (
    <Button
      disabled={!plan || !fingerprint}
      onClick={() => plan && fingerprint && onStart({ answers, fingerprint, plan })}
    >
      {label()}
    </Button>
  )
}
