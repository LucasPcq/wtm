import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { dryRunQuery, questions } from '@/api/queries/commands'
import type { DomainAnswered, DomainForm } from '@/api/types.gen'
import { keyOf, problemsByKey } from '@/components/command-form/answers'
import { FieldInput } from '@/components/command-form/field-input'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldDescription, FieldError, FieldLabel } from '@/components/ui/field'
import { Stepper } from '@/features/worktrees/components/stepper'
import { RemovePlan } from '@/features/worktrees/remove/components/remove-plan'
import type { Start } from '@/features/worktrees/remove/hooks/use-remove-run'
import { worktreeWords } from '@/features/worktrees/remove/utils/words'

export function RemoveWizard({
  repo,
  form,
  preset,
  onStart,
  onCancel,
}: {
  repo: string
  form: DomainForm
  preset: string[]
  onStart: (start: Start) => void
  onCancel: () => void
}) {
  const [picked, setPicked] = useState(preset)
  const [confirming, setConfirming] = useState(preset.length > 0)
  const [reparent, setReparent] = useState(false)
  const [force, setForce] = useState(false)
  const [problem, setProblem] = useState<string>()

  const field = form.fields.find((candidate) => candidate.path === 'worktree')
  const answers: DomainAnswered[] = [
    { path: 'worktree', values: picked },
    ...(reparent ? [{ path: 'reparent_children', values: ['true'] }] : []),
    ...(force ? [{ path: 'force', values: ['true'] }] : []),
  ]
  const dry = useQuery({ ...dryRunQuery(repo, 'clean', answers), enabled: confirming })
  const plan = dry.data?.error ? undefined : dry.data?.plan
  const fingerprint = dry.data?.fingerprint
  const count = plan?.worktrees?.length ?? 0

  // the core says what is missing: Next asks it with the worktrees picked
  const next = async () => {
    const answered = await questions(repo, 'clean', [{ path: 'worktree', values: picked }])
    const refused = problemsByKey(answered.fields, answered.problems)[keyOf('worktree')]
    setProblem(refused)
    if (!refused) setConfirming(true)
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Remove worktrees</DialogTitle>
        <DialogDescription>{worktreeWords.help}</DialogDescription>
      </DialogHeader>

      <div className="border-b pb-4">
        <Stepper
          steps={[
            { title: 'Worktrees', state: confirming ? 'done' : 'now' },
            { title: 'Confirm', state: confirming ? 'now' : 'todo' },
          ]}
        />
      </div>

      {!confirming && field && (
        <Field data-invalid={Boolean(problem)}>
          <FieldLabel htmlFor="worktree">{worktreeWords.label}</FieldLabel>
          <FieldInput
            field={field}
            id="worktree"
            words={worktreeWords}
            from={undefined}
            values={picked}
            onChange={setPicked}
            invalid={Boolean(problem)}
          />
          {problem ? <FieldError>{problem}</FieldError> : <FieldDescription>Which worktrees go.</FieldDescription>}
        </Field>
      )}

      {confirming && (
        <RemovePlan
          repo={repo}
          answers={answers}
          picked={picked}
          reparent={reparent}
          force={force}
          onReparent={setReparent}
          onForce={setForce}
        />
      )}

      <DialogFooter className="items-center">
        <Button variant="ghost" className="mr-auto" onClick={onCancel}>
          Cancel
        </Button>
        {confirming && (
          <Button variant="outline" onClick={() => setConfirming(false)}>
            Back
          </Button>
        )}
        {!confirming && <Button onClick={next}>Next</Button>}
        {confirming && (
          <Button
            variant="destructive"
            disabled={!plan || !fingerprint || count === 0}
            onClick={() => plan && fingerprint && onStart({ answers, fingerprint, plan })}
          >
            {count === 1 ? `Remove ${plan?.worktrees?.[0]?.worktree}` : `Remove ${count} worktrees`}
          </Button>
        )}
      </DialogFooter>
    </>
  )
}
