import { useState } from 'react'
import type { DomainField, DomainForm } from '@/api/types.gen'
import { keyOf, problemsByKey } from '@/components/command-form/answers'
import { useCommandForm } from '@/components/command-form/use-command-form'
import type { StepState } from '@/features/worktrees/components/stepper'
import { isSection, type Section, sectionTitle } from '@/features/worktrees/set-up/utils/words'

// a step per section of init's form, in the core's order, then Confirm; a step's fields are those the core places in it
export function useSetUp({ repo, form: initial }: { repo: string; form: DomainForm }) {
  const commandForm = useCommandForm({ repo, command: 'init', form: initial })
  const { form, asked, check } = commandForm
  const [at, setAt] = useState(0)

  const sections = asked.sections.map((section) => section.id).filter(isSection)
  const confirming = at >= sections.length
  const fieldsOf = (section: Section | undefined) => asked.fields.filter((field) => field.section === section)

  const state = (i: number): StepState => {
    if (i === at) return 'now'
    if (i > at) return 'todo'
    return 'done'
  }

  // Next asks the core with the answers as typed; a step with a problem shows it rather than moving on
  const next = async () => {
    const answered = await check().catch(() => undefined)
    if (!answered) return
    const filed = problemsByKey(answered.fields, answered.problems)
    const here = answered.fields.filter((field) => field.section === sections[at])
    if (!here.some((field) => filed[keyOf(field.path)])) return setAt(at + 1)

    for (const field of here) form.setFieldMeta(keyOf(field.path), (meta) => ({ ...meta, isTouched: true }))
  }

  return {
    ...commandForm,
    at,
    section: sections[at],
    fields: fieldsOf(sections[at]),
    field: <K extends DomainField['kind']>(path: string, kind: K) =>
      asked.fields.find(
        (field): field is Extract<DomainField, { kind: K }> => field.path === path && field.kind === kind,
      ),
    confirming,
    stepper: [
      ...sections.map((section, i) => ({ title: sectionTitle[section], state: state(i) })),
      { title: 'Confirm', state: confirming ? ('now' as const) : ('todo' as const) },
    ],
    back: () => setAt(Math.max(at - 1, 0)),
    next: () => void next(),
  }
}
