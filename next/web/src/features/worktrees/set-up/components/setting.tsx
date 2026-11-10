import type { ReactNode } from 'react'
import type { DomainField } from '@/api/types.gen'
import { defaultOf, keyOf } from '@/components/command-form/answers'
import type { CommandForm } from '@/components/command-form/use-command-form'
import { Field, FieldDescription, FieldError, FieldLabel } from '@/components/ui/field'

export type Input = { id: string; values: string[]; onChange: (values: string[]) => void; invalid: boolean }

// a value the person has not changed is the core's last answer: a strategy turned on brings the files it picks
export function Setting({
  form,
  field,
  label,
  help,
  problems,
  children,
}: {
  form: CommandForm
  field: DomainField
  label: string
  help?: ReactNode
  problems: Record<string, string>
  children: (input: Input) => ReactNode
}) {
  return (
    <form.Field name={keyOf(field.path)} defaultValue={defaultOf(field)}>
      {(input) => {
        const { isTouched, isDirty } = input.state.meta
        const problem = isTouched ? problems[input.name] : undefined
        const values = isDirty ? (input.state.value ?? defaultOf(field)) : defaultOf(field)

        return (
          <Field data-invalid={problem !== undefined} className="gap-2">
            <FieldLabel htmlFor={input.name}>{label}</FieldLabel>
            {children({ id: input.name, values, onChange: input.handleChange, invalid: problem !== undefined })}
            {help && <FieldDescription className="text-xs text-pretty">{help}</FieldDescription>}
            <FieldError errors={problem?.split('\n').map((message) => ({ message })) ?? []} />
          </Field>
        )
      }}
    </form.Field>
  )
}
