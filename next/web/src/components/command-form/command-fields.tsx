import type { DomainField, DomainOrigin } from '@/api/types.gen'
import { defaultOf, keyOf } from '@/components/command-form/answers'
import { FieldInput } from '@/components/command-form/field-input'
import type { CommandForm } from '@/components/command-form/use-command-form'
import { Checkbox } from '@/components/ui/checkbox'
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'

// The core gives no words: a screen words each field, through an exhaustive Record over its command's paths.
export type OptionWords = { title: string; description: string }
export type FieldWords = { label: string; help: string; options?: Record<string, OptionWords>; config?: string }

const origin: Record<DomainOrigin, string | undefined> = {
  request: undefined,
  default: undefined,
  remembered: 'remembered',
  config: 'from config',
  pull_request: 'from the pull request',
}

export type Remember = { paths: string[]; toggle: (path: string) => void }

export function CommandFields<P extends string>({
  form,
  fields,
  words,
  problems,
  remember,
}: {
  form: CommandForm
  fields: (DomainField & { path: P })[]
  words: Record<P, FieldWords>
  problems: Record<string, string>
  remember?: Remember
}) {
  return (
    <FieldGroup className="gap-6">
      {fields.map((field) => {
        const said = words[field.path]
        const rememberable = remember && 'rememberable' in field && field.rememberable

        const from = (dirty: boolean) => {
          if (!field.origin || dirty) return undefined
          if (field.origin === 'config') return said.config ?? origin.config
          return origin[field.origin]
        }

        return (
          <form.Field key={field.path} name={keyOf(field.path)} defaultValue={defaultOf(field)}>
            {(input) => {
              const { isTouched, isDirty } = input.state.meta
              // a field's problems show once the person touched it, not when the form opens empty
              const problem = isTouched ? problems[input.name] : undefined
              const errors = problem ? problem.split('\n').map((message) => ({ message })) : []

              return (
                <Field data-invalid={errors.length > 0} className="gap-2">
                  <div className="flex flex-col gap-1">
                    <div className="flex items-center justify-between gap-2">
                      <FieldLabel htmlFor={input.name}>{said.label}</FieldLabel>
                      {rememberable && (
                        <label
                          htmlFor={`${input.name}-remember`}
                          className="flex items-center gap-1.5 text-xs text-muted-foreground"
                        >
                          <Checkbox
                            id={`${input.name}-remember`}
                            className="size-3.5"
                            checked={remember.paths.includes(field.path)}
                            onCheckedChange={() => remember.toggle(field.path)}
                          />
                          Remember
                        </label>
                      )}
                    </div>
                    <FieldDescription className="text-xs text-pretty">{said.help}</FieldDescription>
                  </div>
                  <FieldInput
                    field={field}
                    id={input.name}
                    words={said}
                    from={from(isDirty)}
                    values={isDirty ? (input.state.value ?? defaultOf(field)) : defaultOf(field)}
                    onChange={(values) => input.handleChange(values)}
                    invalid={errors.length > 0}
                  />
                  <FieldError errors={errors} />
                </Field>
              )
            }}
          </form.Field>
        )
      })}
    </FieldGroup>
  )
}
