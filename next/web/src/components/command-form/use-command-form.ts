import { useForm } from '@tanstack/react-form'
import { useRef, useState } from 'react'
import { questions } from '@/api/queries/commands'
import type { ApiCommandName, DomainAnswered, DomainForm } from '@/api/types.gen'
import { answersOf, problemsByKey, valuesOf } from '@/components/command-form/answers'

// The core validates: a change asks the questions again once typing pauses, check asks them at once, and the
// core's answer is the form as it stands now (a field it no longer asks, a default that followed another answer).
export function useCommandForm(params: { repo: string; command: ApiCommandName; form: DomainForm }) {
  const [asked, setAsked] = useState(params.form)
  // each call is numbered: an answer overtaken by a later call's is dropped, never kept over it
  const calls = useRef(0)
  const form = useForm({
    defaultValues: valuesOf(params.form.fields),
    validators: {
      onChangeAsyncDebounceMs: 300,
      onChangeAsync: async () => {
        await check()
        return undefined
      },
    },
  })

  // answers is the request the form makes now: only the fields the person changed
  const answers = (): DomainAnswered[] =>
    answersOf(asked.fields, form.state.values, (key) => form.getFieldMeta(key)?.isDirty ?? false)
  const check = async () => {
    const call = ++calls.current
    const answered = await questions(params.repo, params.command, answers())
    if (call === calls.current) setAsked(answered)
    return answered
  }
  return { form, asked, answers, check, problems: problemsByKey(asked.fields, asked.problems) }
}

export type CommandForm = ReturnType<typeof useCommandForm>['form']
