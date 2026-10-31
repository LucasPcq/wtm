import { useForm } from '@tanstack/react-form'
import { useRef, useState } from 'react'
import { questions } from '@/api/queries/commands'
import type { ApiCommandName, DomainAnswered, DomainForm } from '@/api/types.gen'
import { answersOf, problemsByKey, valuesOf } from '@/components/command-form/answers'

// The core validates: a change asks the questions again once typing pauses, check asks them at once, and the
// core's answer is the form as it stands now (a field it no longer asks, a default that followed another answer).
export function useCommandForm({
  repo,
  command,
  form: initial,
  extend = (answers) => answers,
  onAnswered,
}: {
  repo: string
  command: ApiCommandName
  form: DomainForm
  extend?: (answers: DomainAnswered[]) => DomainAnswered[]
  onAnswered?: (sent: DomainAnswered[]) => void
}) {
  const [asked, setAsked] = useState(initial)
  // each call is numbered: an answer overtaken by a later call's is dropped, never kept over it
  const calls = useRef(0)

  const form = useForm({
    defaultValues: valuesOf(initial.fields),
    validators: {
      onChangeAsyncDebounceMs: 300,
      onChangeAsync: async () => {
        await check()
        return undefined
      },
    },
  })

  // only the fields the person changed: a default or a remembered answer stays the core's to decide
  const answers = (): DomainAnswered[] =>
    extend(answersOf(asked.fields, form.state.values, (key) => form.getFieldMeta(key)?.isDirty ?? false))

  const check = async () => {
    const call = ++calls.current
    const sent = answers()
    const answered = await questions(repo, command, sent)
    onAnswered?.(sent)
    if (call === calls.current) setAsked(answered)
    return answered
  }

  return { form, asked, answers, check, problems: problemsByKey(asked.fields, asked.problems) }
}

export type CommandForm = ReturnType<typeof useCommandForm>['form']
