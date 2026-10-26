import type { DomainField } from '@/api/types.gen'
import { ChoiceOptions } from '@/components/choice-options'
import type { FieldWords } from '@/components/command-form/command-fields'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

export function FieldInput({
  field,
  id,
  words,
  from,
  values,
  onChange,
  invalid,
}: {
  field: DomainField
  id: string
  words: FieldWords
  from: string | undefined
  values: string[]
  onChange: (values: string[]) => void
  invalid: boolean
}) {
  const title = (value: string) => words.options?.[value]?.title ?? value
  const line = (value: string) => words.options?.[value]?.description

  switch (field.kind) {
    case 'select': {
      const picked = field.options.find((option) => option.value === values[0])
      return (
        <Select value={values[0] ?? ''} onValueChange={(value) => onChange([value])}>
          <SelectTrigger id={id} aria-invalid={invalid} className="w-full">
            {/* the closed select names its answer and where it came from, never the option's description */}
            <SelectValue>
              {picked && title(picked.value)}
              {picked && from && <span className="text-muted-foreground"> · {from}</span>}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {field.options.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                <span className="flex flex-col">
                  {title(option.value)}
                  {line(option.value) && <span className="text-xs text-muted-foreground">{line(option.value)}</span>}
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
            <label key={option.value} htmlFor={`${id}-${option.value}`} className="flex items-center gap-2 text-sm">
              <Checkbox
                id={`${id}-${option.value}`}
                checked={values.includes(option.value)}
                onCheckedChange={(on) =>
                  onChange(on ? [...values, option.value] : values.filter((value) => value !== option.value))
                }
              />
              {title(option.value)}
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
      const options = field.no_first ? ['false', 'true'] : ['true', 'false']
      return (
        <div className="flex flex-col gap-2">
          <ChoiceOptions
            id={id}
            value={values[0] ?? ''}
            options={options.map((value) => ({ value, title: title(value), description: line(value) }))}
            onChange={(value) => onChange([value])}
            invalid={invalid}
          />
          {from && <span className="text-xs text-muted-foreground">{from}</span>}
        </div>
      )
    }
  }
}
