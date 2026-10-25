import type { DomainField, DomainOption, DomainOrigin } from '@/api/types.gen'
import { Checkbox } from '@/components/ui/checkbox'
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { keyOf, valueOf } from './answers'
import type { CommandForm } from './use-command-form'

// The core's labels and helps are the command line's; a screen gives each field its own words, by path, and each
// option of a closed set a title and a line. An option of no words is data, a branch: it reads as its value, the
// core's label only for the empty one.
export type OptionWords = { title: string; description: string }
export type FieldWords = { label: string; help: string; options?: Record<string, OptionWords>; config?: string }

// Where an answer came from, said beside it; one the person gave, or the field's default, says nothing.
const origin: Record<DomainOrigin, string | undefined> = {
  request: undefined,
  default: undefined,
  remembered: 'remembered',
  config: 'from config',
}

// Remember is which answers are to be remembered, by path, and how to change that: the fields that may be
// remembered offer it.
export type Remember = { paths: string[]; toggle: (path: string) => void }

// CommandFields draws each field of a command's form in its screen's words: its label, then its answer, then what it
// is for, then what else it offers; its problems, the core's, once touched. One the person has not touched shows the
// core's answer as it stands now, and where it came from.
export function CommandFields(props: {
  form: CommandForm
  fields: DomainField[]
  words: Record<string, FieldWords>
  problems: Record<string, string>
  remember?: Remember
}) {
  return (
    <FieldGroup className="gap-6">
      {props.fields.map((field) => {
        const words = props.words[field.path]
        return (
          <props.form.Field key={field.path} name={keyOf(field.path)} defaultValue={valueOf(field)}>
            {(input) => {
              // a field's problems show once the person touched it, not when the form opens empty
              const problem = input.state.meta.isTouched ? props.problems[input.name] : undefined
              const errors = problem ? problem.split('\n').map((message) => ({ message })) : []
              const from =
                field.origin && !input.state.meta.isDirty
                  ? field.origin === 'config'
                    ? (words?.config ?? origin.config)
                    : origin[field.origin]
                  : undefined
              const rememberable = props.remember && 'rememberable' in field && field.rememberable
              return (
                <Field data-invalid={errors.length > 0} className="gap-2">
                  <div className="flex items-center justify-between gap-2">
                    <FieldLabel htmlFor={input.name}>{words?.label ?? field.label}</FieldLabel>
                    {rememberable && (
                      <label className="flex items-center gap-1.5 text-xs text-muted-foreground">
                        <Checkbox
                          className="size-3.5"
                          checked={props.remember?.paths.includes(field.path)}
                          onCheckedChange={() => props.remember?.toggle(field.path)}
                        />
                        Remember
                      </label>
                    )}
                  </div>
                  <FieldInput
                    field={field}
                    id={input.name}
                    words={words}
                    from={from}
                    values={input.state.meta.isDirty ? (input.state.value ?? valueOf(field)) : valueOf(field)}
                    onChange={(values) => input.handleChange(values)}
                    invalid={errors.length > 0}
                  />
                  <FieldDescription className="text-sm text-pretty">{words?.help ?? field.help}</FieldDescription>
                  <FieldError errors={errors} />
                </Field>
              )
            }}
          </props.form.Field>
        )
      })}
    </FieldGroup>
  )
}

type InputProps = {
  field: DomainField
  id: string
  words: FieldWords | undefined
  from: string | undefined
  values: string[]
  onChange: (values: string[]) => void
  invalid: boolean
}

function FieldInput({ field, id, words, from, values, onChange, invalid }: InputProps) {
  const title = (option: DomainOption) => words?.options?.[option.value]?.title ?? (option.value || option.label)
  const line = (option: DomainOption) => words?.options?.[option.value]?.description
  switch (field.kind) {
    case 'select': {
      const picked = field.options.find((option) => option.value === values[0])
      return (
        <Select value={values[0] ?? ''} onValueChange={(value) => onChange([value])}>
          <SelectTrigger id={id} aria-invalid={invalid} className="w-full">
            {/* the closed select names its answer and where it came from, never the option's description */}
            <SelectValue>
              {picked && title(picked)}
              {picked && from && <span className="text-muted-foreground"> · {from}</span>}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {field.options.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                <span className="flex flex-col">
                  {title(option)}
                  {line(option) && <span className="text-xs text-muted-foreground">{line(option)}</span>}
                </span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )
    }
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
              {title(option)}
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
            <label key={option.value} className="flex items-start gap-2 text-sm">
              <RadioGroupItem value={option.value} className="mt-0.5" />
              <span className="flex flex-col">
                {title(option)}
                {line(option) && <span className="text-xs text-muted-foreground">{line(option)}</span>}
              </span>
            </label>
          ))}
          {from && <span className="text-xs text-muted-foreground">{from}</span>}
        </RadioGroup>
      )
    }
  }
}
