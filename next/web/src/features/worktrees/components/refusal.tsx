import type { DomainError } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Alert } from '@/features/worktrees/components/alert'
import { CommandLine } from '@/features/worktrees/components/command-line'

type Consent = { label: string; to: 'force' | 'exclusive' }

// the follow-ups of a refusal the person consents to on Confirm, by their code; any other is a command to copy
const consents: Record<`${'run-up' | 'run-start'}.${'force' | 'exclusive'}`, Consent> = {
  'run-up.force': { label: 'Start anyway', to: 'force' },
  'run-up.exclusive': { label: "Stop the others' jobs", to: 'exclusive' },
  'run-start.force': { label: 'Start anyway', to: 'force' },
  'run-start.exclusive': { label: "Stop the others' jobs", to: 'exclusive' },
}

const consented = (code: string | undefined): code is keyof typeof consents =>
  code !== undefined && Object.hasOwn(consents, code)

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
  const code = refusal?.follow_up?.code
  const lift = consented(code) ? consents[code] : undefined

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
