import type { DomainField } from '@/api/types.gen'
import { Checkbox } from '@/components/ui/checkbox'
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { keyOf, valueOf } from './answers'
import type { CommandForm } from './use-command-form'

// Where an answer came from, said beside its field; one the person gave says nothing.
const origin: Record<string, string> = { config: 'from config', remembered: 'remembered', default: 'default' }

// Remember is which answers are to be remembered, by path, and how to change that: the fields that may be
// remembered offer it.
export type Remember = { paths: string[]; toggle: (path: string) => void }

// CommandFields draws each field of a command's form with the input of its kind; one the person has not touched
// shows the core's answer as it stands now, and where it came from.
export function CommandFields(props: { form: CommandForm; fields: DomainField[]; remember?: Remember }) {
  return (
    <FieldGroup>
      {props.fields.map((field) => (
        <props.form.Field key={field.path} name={keyOf(field.path)} defaultValue={valueOf(field)}>
          {(input) => {
            const errors = input.state.meta.errors.filter(Boolean).map((message) => ({ message: String(message) }))
            const set = (values: string[]) => input.handleChange(values)
            return (
              <Field data-invalid={errors.length > 0}>
                <div className="flex items-baseline justify-between gap-2">
                  <FieldLabel htmlFor={input.name}>{field.label}</FieldLabel>
                  {field.origin && !input.state.meta.isDirty && (
                    <span className="text-xs text-muted-foreground">{origin[field.origin]}</span>
                  )}
                </div>
                <FieldInput
                  field={field}
                  id={input.name}
                  values={input.state.meta.isDirty ? (input.state.value ?? valueOf(field)) : valueOf(field)}
                  onChange={set}
                  invalid={errors.length > 0}
                />
                {field.help && <FieldDescription>{field.help}</FieldDescription>}
                {props.remember && 'rememberable' in field && field.rememberable && (
                  <label className="flex items-center gap-2 text-sm">
                    <Checkbox
                      checked={props.remember.paths.includes(field.path)}
                      onCheckedChange={() => props.remember?.toggle(field.path)}
                    />
                    Always use this answer
                  </label>
                )}
                <FieldError errors={errors} />
              </Field>
            )
          }}
        </props.form.Field>
      ))}
    </FieldGroup>
  )
}

type InputProps = {
  field: DomainField
  id: string
  values: string[]
  onChange: (values: string[]) => void
  invalid: boolean
}

function FieldInput({ field, id, values, onChange, invalid }: InputProps) {
  switch (field.kind) {
    case 'select':
      return (
        <Select value={values[0] ?? ''} onValueChange={(value) => onChange([value])}>
          <SelectTrigger id={id} aria-invalid={invalid}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {field.options.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )
    case 'multi_select':
      return (
        <div id={id} className="flex flex-col gap-2">
          {field.options.map((option) => (
            <label key={option.value} className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={values.includes(option.value)}
                onCheckedChange={(on) =>
                  onChange(on ? [...values, option.value] : values.filter((value) => value !== option.value))
                }
              />
              {option.label}
            </label>
          ))}
        </div>
      )
    case 'text':
      return (
        <Input
          id={id}
          aria-invalid={invalid}
          value={values[0] ?? ''}
          onChange={(e) => onChange(e.target.value ? [e.target.value] : [])}
        />
      )
    case 'text_list':
      return (
        <Textarea id={id} aria-invalid={invalid} value={values[0] ?? ''} onChange={(e) => onChange([e.target.value])} />
      )
    case 'confirm': {
      const options = field.no_first ? [field.no, field.yes] : [field.yes, field.no]
      return (
        <RadioGroup id={id} value={values[0] ?? ''} onValueChange={(value) => onChange([value])} aria-invalid={invalid}>
          {options.map((option) => (
            <label key={option.value} className="flex items-center gap-2 text-sm">
              <RadioGroupItem value={option.value} />
              {option.label}
            </label>
          ))}
        </RadioGroup>
      )
    }
  }
}
