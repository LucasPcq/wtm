import type { DomainError } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Alert } from '@/features/worktrees/components/alert'
import { CommandLine } from '@/features/worktrees/components/command-line'

type Consent = { label: string; to: 'force' | 'exclusive' }

// a refusal the person can lift is never a dead end: their consent plans again, and Confirm shows the new plan before
// anything starts
export function Refusal({
  refusal,
  message,
  onConsent,
}: {
  refusal: DomainError | undefined
  message: string
  onConsent: (to: Consent['to']) => void
}) {
  const consent = (): Consent | undefined => {
    if (refusal?.code === 'run-up.unsafe') return { label: 'Start anyway', to: 'force' }
    // stopping the others lifts a clash only when the core says so: a port a job outside the repository holds stays
    if (refusal?.code === 'run-up.port_clash' && refusal.follow_up)
      return { label: "Stop the others' jobs", to: 'exclusive' }
    return undefined
  }
  const lift = consent()

  return (
    <Alert
      tone="red"
      description={message}
      action={
        lift && (
          <Button size="sm" variant="outline" onClick={() => onConsent(lift.to)}>
            {lift.label}
          </Button>
        )
      }
      below={!lift && refusal?.follow_up && <CommandLine followUp={refusal.follow_up} />}
    />
  )
}
