import { useForm } from '@tanstack/react-form'
import { useState } from 'react'
import { questions } from '@/api/queries/commands'
import type { ApiCommandName, DomainAnswered, DomainForm } from '@/api/types.gen'
import { answersOf, problemsOf, valuesOf, type Values } from './answers'

// useCommandForm is the form of a command as the core describes it (POST …/questions). The core validates: each
// change asks the questions again, files its problems under their fields, and its answer becomes the form asked:
// the fields as they stand now (one it no longer asks, a default that followed another answer).
export function useCommandForm(params: {
  repo: string
  command: ApiCommandName
  form: DomainForm
  onSubmit: (values: Values) => void
}) {
  const [asked, setAsked] = useState(params.form)
  const form = useForm({
    defaultValues: valuesOf(params.form.fields),
    validators: {
      onChangeAsyncDebounceMs: 300,
      onChangeAsync: ({ value, formApi }) =>
        problemsOf({
          fields: asked.fields,
          values: value,
          changed: (key) => formApi.getFieldMeta(key)?.isDirty ?? false,
          ask: (answers) =>
            questions(params.repo, params.command, answers).then((answered) => {
              setAsked(answered)
              return answered
            }),
        }),
    },
    onSubmit: ({ value }) => params.onSubmit(value),
  })

  // answers is the request the form makes now: only the fields the person changed
  const answers = (): DomainAnswered[] =>
    answersOf(asked.fields, form.state.values, (key) => form.getFieldMeta(key)?.isDirty ?? false)
  return { form, asked, answers }
}

export type CommandForm = ReturnType<typeof useCommandForm>['form']
