import { useForm } from '@tanstack/react-form'
import { useRef, useState } from 'react'
import { questions } from '@/api/queries/commands'
import type { ApiCommandName, DomainAnswered, DomainForm } from '@/api/types.gen'
import { answersOf, problemsByKey, valuesOf } from './answers'

// useCommandForm is the form of a command as the core describes it (POST …/questions). The core validates: a
// change asks the questions again once typing pauses, and check asks them at once; the core's answer becomes the
// form asked, the fields as they stand now (one it no longer asks, a default that followed another answer) and
// their problems.
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
