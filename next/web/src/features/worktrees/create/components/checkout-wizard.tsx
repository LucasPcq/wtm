import { type ReactNode, useRef } from 'react'
import type { DomainAnswered, DomainForm } from '@/api/types.gen'
import { CommandFields } from '@/components/command-form/command-fields'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { NextButton } from '@/features/worktrees/components/next-button'
import { Stepper } from '@/features/worktrees/components/stepper'
import { ApplyButton } from '@/features/worktrees/create/components/apply-button'
import { CheckoutConfirm } from '@/features/worktrees/create/components/checkout-confirm'
import { NotAsked } from '@/features/worktrees/create/components/not-asked'
import { ParentPlaceholder } from '@/features/worktrees/create/components/parent-placeholder'
import { PrPicker } from '@/features/worktrees/create/components/pr-picker'
import { useWizard } from '@/features/worktrees/create/hooks/use-wizard'
import { checkoutSteps, checkoutWords } from '@/features/worktrees/create/utils/checkout-words'
import type { Start } from '@/features/worktrees/hooks/use-run'

const noFetch: DomainAnswered = { path: 'no_fetch', values: ['true'] }
const prOf = (answers: DomainAnswered[]) => answers.find((answer) => answer.path === 'pr')?.values[0]

export function CheckoutWizard({
  repo,
  form: initial,
  startFrom,
  onStart,
  onCancel,
  onNewBranch,
}: {
  repo: string
  form: DomainForm
  startFrom: ReactNode
  onStart: (start: Start) => void
  onCancel: () => void
  onNewBranch: () => void
}) {
  // a pull request's branch is fetched by the first ask that names it; every later one, the plan and the run read it
  // as then, so a refresh of the dialog never fetches again
  const fetched = useRef(new Set<string>())
  const { form, asked, problems, check, at, step, confirming, request, stepper, remember, back, next } = useWizard({
    repo,
    command: 'checkout',
    form: initial,
    stepOf: checkoutSteps,
    extend: (answers) => {
      const pr = prOf(answers)
      return pr && fetched.current.has(pr) ? [...answers, noFetch] : answers
    },
    onAnswered: (sent) => {
      const pr = prOf(sent)
      if (pr) fetched.current.add(pr)
    },
  })

  const field = asked.fields.find((candidate) => candidate.path === 'pr')
  const options = field?.kind === 'select' ? field.options : []
  const chosen = (field?.kind === 'select' && field.value) || undefined
  const option = options.find((candidate) => candidate.value === chosen)
  const refusals = chosen ? asked.problems.filter((problem) => problem.path === 'pr') : []
  const words = checkoutWords(option)

  const blocked = () => {
    if (at > 0) return undefined
    if (!chosen) return 'Choose a pull request'
    if (refusals.length > 0) return `#${chosen} can't be checked out: choose another`
    return undefined
  }
  const why = blocked()

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
        {step && at === 0 && (
          <div className="flex flex-col gap-6">
            {startFrom}
            <PrPicker
              form={form}
              options={options}
              chosen={chosen}
              errors={refusals}
              onAsk={() => void check()}
              onNewBranch={onNewBranch}
            />
            {chosen ? (
              <CommandFields
                form={form}
                fields={step.asked.filter((asking) => asking.path !== 'pr')}
                words={words}
                problems={problems}
                remember={remember}
              />
            ) : (
              <ParentPlaceholder />
            )}
          </div>
        )}

        {step && at > 0 && (
          <div className="flex flex-col gap-6">
            <CommandFields form={form} fields={step.asked} words={words} problems={problems} remember={remember} />
            {problems[''] && <p className="text-sm text-destructive">{problems['']}</p>}
            <NotAsked fields={step.skipped} words={words} />
          </div>
        )}

        {confirming && <CheckoutConfirm repo={repo} answers={request} />}
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
            {(validating) => <NextButton disabled={validating || why !== undefined} why={why} onClick={next} />}
          </form.Subscribe>
        )}
        {confirming && <ApplyButton repo={repo} command="checkout" answers={request} onStart={onStart} />}
      </DialogFooter>
    </>
  )
}
