import type { ReactNode } from 'react'
import type { DomainForm } from '@/api/types.gen'
import { CommandFields } from '@/components/command-form/command-fields'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { Stepper } from '@/features/worktrees/components/stepper'
import { ApplyButton } from '@/features/worktrees/create/components/apply-button'
import { Confirm } from '@/features/worktrees/create/components/confirm'
import { NotAsked } from '@/features/worktrees/create/components/not-asked'
import { useWizard } from '@/features/worktrees/create/hooks/use-wizard'
import { createSteps } from '@/features/worktrees/create/utils/steps'
import { createWords } from '@/features/worktrees/create/utils/words'
import type { Start } from '@/features/worktrees/hooks/use-run'

export function Wizard({
  repo,
  form: initial,
  startFrom,
  onStart,
  onCancel,
}: {
  repo: string
  form: DomainForm
  startFrom: ReactNode
  onStart: (start: Start) => void
  onCancel: () => void
}) {
  const { form, problems, at, step, confirming, request, stepper, remember, back, next } = useWizard({
    repo,
    command: 'create',
    form: initial,
    stepOf: createSteps,
  })

  return (
    <>
      <DialogHeader>
        <DialogTitle>Create worktrees</DialogTitle>
        <DialogDescription>Each branch gets its own folder, .env and ports.</DialogDescription>
      </DialogHeader>

      <DialogBand>
        <Stepper steps={stepper} />
      </DialogBand>

      <DialogScroll>
        {step && (
          <div className="flex flex-col gap-6">
            {at === 0 && startFrom}
            <CommandFields
              form={form}
              fields={step.asked}
              words={createWords}
              problems={problems}
              remember={remember}
            />
            {problems[''] && <p className="text-sm text-destructive">{problems['']}</p>}
            <NotAsked fields={step.skipped} words={createWords} />
          </div>
        )}

        {confirming && <Confirm repo={repo} answers={request} />}
      </DialogScroll>

      <DialogFooter className="items-center">
        <Button variant="ghost" className="mr-auto" onClick={onCancel}>
          Cancel
        </Button>
        {at > 0 && (
          <Button variant="outline" onClick={back}>
            Back
          </Button>
        )}
        {!confirming && (
          <form.Subscribe selector={(s) => s.isValidating}>
            {(validating) => (
              <Button disabled={validating} onClick={next}>
                Next
              </Button>
            )}
          </form.Subscribe>
        )}
        {confirming && <ApplyButton repo={repo} command="create" answers={request} onStart={onStart} />}
      </DialogFooter>
    </>
  )
}
