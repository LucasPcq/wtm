import { useForm } from '@tanstack/react-form'
import { questions } from '@/api/queries'
import type { DomainForm } from '@/api/types.gen'
import { problemsOf, valuesOf, type Values } from './answers'

// useCommandForm is the form of a command as the core describes it (POST …/questions). The core validates: each
// change asks the questions again and files its problems under their fields.
export function useCommandForm(params: { repo: string; command: string; form: DomainForm; onSubmit: (values: Values) => void }) {
  const { fields } = params.form
  return useForm({
    defaultValues: valuesOf(fields),
    validators: {
      onChangeAsyncDebounceMs: 300,
      onChangeAsync: ({ value, formApi }) =>
        problemsOf({
          fields,
          values: value,
          changed: (key) => formApi.getFieldMeta(key)?.isDirty ?? false,
          ask: (answers) => questions(params.repo, params.command, answers),
        }),
    },
    onSubmit: ({ value }) => params.onSubmit(value),
  })
}

export type CommandForm = ReturnType<typeof useCommandForm>
