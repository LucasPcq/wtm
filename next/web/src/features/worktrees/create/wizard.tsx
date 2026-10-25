import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { dryRunQuery } from '@/api/queries/commands'
import type { CreatePlan, DomainAnswered, DomainForm } from '@/api/types.gen'
import { CommandFields } from '@/components/command-form/command-fields'
import { keyOf, problemsByKey } from '@/components/command-form/answers'
import { useCommandForm } from '@/components/command-form/use-command-form'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import type { Start } from '../hooks/use-create-run'
import { Stepper, type StepState } from './stepper'
import { stepsOf } from './steps'
import { createWords, labelOf, skipWords } from './words'

const remembered = (form: DomainForm) =>
  form.fields.filter((field) => 'remember' in field && field.remember).map((field) => field.path)

// Wizard asks create's questions a step at a time, as the core groups them and asks them again after each answer,
// then shows what the dry run would do, and starts it held to its fingerprint.
export function Wizard(props: {
  repo: string
  form: DomainForm
  onStart: (start: Start) => void
  onCancel: () => void
}) {
  const { form, asked, answers, check, problems } = useCommandForm({
    repo: props.repo,
    command: 'create',
    form: props.form,
  })
  const [at, setAt] = useState(0)
  const [remember, setRemember] = useState(() => remembered(props.form))
  const steps = stepsOf(asked)
  const confirming = at >= steps.length
  const request: DomainAnswered[] = [
    ...answers(),
    ...(remember.length > 0 ? [{ path: 'remember', values: remember }] : []),
  ]

  const step = steps[at]
  // only a reason that tells the person something; one that holds on the command line alone is not said
  const notAsked = (step?.skipped ?? []).flatMap((field) => {
    const reason = field.skip && skipWords[field.skip]
    return reason ? [{ path: field.path, reason }] : []
  })
  // a step the core asks nothing of is passed over, as not asked
  const next = (from: number) => {
    const ahead = steps.findIndex((candidate, i) => i > from && candidate.asked.length > 0)
    return ahead < 0 ? steps.length : ahead
  }
  const previous = steps.findLastIndex((candidate, i) => i < at && candidate.asked.length > 0)
  const state = (i: number): StepState =>
    i === at ? 'now' : steps[i]?.asked.length === 0 && i < at ? 'skipped' : i < at ? 'done' : 'todo'

  return (
    <>
      <DialogHeader>
        <DialogTitle>Create worktree</DialogTitle>
        <DialogDescription>Each branch gets its own folder, .env and ports.</DialogDescription>
      </DialogHeader>
      <div className="border-b pb-4">
        <Stepper
          steps={[
            ...steps.map((s, i) => ({ title: s.title, state: state(i) })),
            { title: 'Confirm', state: confirming ? 'now' : 'todo' },
          ]}
        />
      </div>
      {step && (
        <div className="flex flex-col gap-6">
          <CommandFields
            form={form}
            fields={step.asked}
            words={createWords}
            problems={problems}
            remember={{
              paths: remember,
              toggle: (path) =>
                setRemember((paths) => (paths.includes(path) ? paths.filter((p) => p !== path) : [...paths, path])),
            }}
          />
          {problems[''] && <p className="text-sm text-destructive">{problems['']}</p>}
          {notAsked.length > 0 && (
            <ul className="border-t pt-3 text-xs text-muted-foreground">
              {notAsked.map(({ path, reason }) => (
                <li key={path}>
                  Not asked: {labelOf(path)}, {reason}.
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
      {confirming && <Confirm repo={props.repo} answers={request} />}
      <DialogFooter className="items-center">
        <Button variant="ghost" className="mr-auto" onClick={props.onCancel}>
          Cancel
        </Button>
        {at > 0 && (
          <Button variant="outline" onClick={() => setAt(previous < 0 ? 0 : previous)}>
            Back
          </Button>
        )}
        {!confirming && (
          <form.Subscribe selector={(s) => s.isValidating}>
            {(validating) => (
              <Button
                disabled={validating}
                onClick={async () => {
                  // Next asks the core with the answers as typed, not as last checked: a step with problems shows
                  // them, rather than a Next that cannot be pressed
                  const answered = await check()
                  const filed = problemsByKey(answered.fields, answered.problems)
                  const here = stepsOf(answered)[at]
                  if (!here?.asked.some((field) => filed[keyOf(field.path)])) return setAt(next(at))
                  for (const field of here.asked)
                    form.setFieldMeta(keyOf(field.path), (meta) => ({ ...meta, isTouched: true }))
                }}
              >
                Next
              </Button>
            )}
          </form.Subscribe>
        )}
        {confirming && <ApplyButton repo={props.repo} answers={request} onStart={props.onStart} />}
      </DialogFooter>
    </>
  )
}

// applyLabel names the apply by the worktrees it opens, as the command line's wizard does.
function applyLabel(plan: CreatePlan | undefined): string {
  if (!plan || plan.creates === 0) return 'Apply'
  return plan.creates === 1 ? 'Create worktree' : `Create ${plan.creates} worktrees`
}

function ApplyButton(props: { repo: string; answers: DomainAnswered[]; onStart: (start: Start) => void }) {
  const dry = useQuery(dryRunQuery(props.repo, 'create', props.answers))
  const plan = dry.data?.plan as CreatePlan | undefined
  const fingerprint = dry.data?.fingerprint
  return (
    <Button
      disabled={!plan || !fingerprint}
      onClick={() => plan && fingerprint && props.onStart({ answers: props.answers, fingerprint, plan })}
    >
      {applyLabel(plan)}
    </Button>
  )
}

// Confirm is the dry run's recap: what the apply will do, each row as the core words it.
function Confirm(props: { repo: string; answers: DomainAnswered[] }) {
  const dry = useQuery(dryRunQuery(props.repo, 'create', props.answers))
  if (dry.isPending) return <Skeleton className="h-40 w-full" />
  if (dry.isError) return <p className="text-sm text-destructive">{dry.error.message}</p>
  const plan = dry.data.plan as CreatePlan | undefined
  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm">
        wtm will create {plan?.creates ?? 0} {plan?.creates === 1 ? 'worktree' : 'worktrees'}:
      </p>
      <dl className="divide-y rounded-lg border text-sm">
        {(plan?.recap ?? []).map((row) => (
          <div key={row.key} className="grid grid-cols-[7rem_1fr] gap-4 px-3.5 py-2.5">
            <dt className="text-muted-foreground">{row.label}</dt>
            <dd className="flex flex-col">
              {(row.values ?? []).map((value) => (
                <span key={value.value + value.text}>
                  {value.text}
                  {value.note && <span className="block text-xs text-muted-foreground">{value.note}</span>}
                </span>
              ))}
            </dd>
          </div>
        ))}
      </dl>
      {dry.data.warnings.map((warning) => (
        <p key={warning.code} className="text-sm text-amber-600 dark:text-amber-500">
          {warning.message}
        </p>
      ))}
    </div>
  )
}
