import type { DomainField } from '@/api/types.gen'
import { Checkbox } from '@/components/ui/checkbox'
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { keyOf, valueOf } from './answers'
import type { CommandForm } from './use-command-form'

// CommandFields draws each field of a command's form with the input of its kind.
export function CommandFields(props: { form: CommandForm; fields: DomainField[] }) {
  return (
    <FieldGroup>
      {props.fields.map((field) => (
        <props.form.Field key={field.path} name={keyOf(field.path)} defaultValue={valueOf(field)}>
          {(input) => {
            const errors = input.state.meta.errors.filter(Boolean).map((message) => ({ message: String(message) }))
            const set = (values: string[]) => input.handleChange(values)
            return (
              <Field data-invalid={errors.length > 0}>
                {field.kind !== 'confirm' && <FieldLabel htmlFor={input.name}>{field.label}</FieldLabel>}
                <FieldInput
                  field={field}
                  id={input.name}
                  values={input.state.value ?? valueOf(field)}
                  onChange={set}
                  invalid={errors.length > 0}
                />
                {field.help && <FieldDescription>{field.help}</FieldDescription>}
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
    case 'confirm':
      return (
        <label className="flex items-center gap-2 text-sm">
          <Checkbox
            id={id}
            checked={values[0] === field.yes.value}
            onCheckedChange={(on) => onChange([on ? field.yes.value : field.no.value])}
          />
          {field.label}
        </label>
      )
  }
}
