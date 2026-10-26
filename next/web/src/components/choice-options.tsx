import { Field, FieldContent, FieldDescription, FieldLabel } from '@/components/ui/field'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'

export type ChoiceOption = { value: string; title: string; description?: string | undefined }

export function ChoiceOptions({
  id,
  label,
  value,
  options,
  onChange,
  invalid = false,
}: {
  id: string
  label?: string
  value: string
  options: ChoiceOption[]
  onChange: (value: string) => void
  invalid?: boolean
}) {
  return (
    <RadioGroup
      id={id}
      aria-label={label}
      value={value}
      onValueChange={onChange}
      aria-invalid={invalid}
      className="gap-4"
    >
      {options.map((option) => (
        <Field key={option.value} orientation="horizontal" className="items-start gap-3">
          <RadioGroupItem id={`${id}-${option.value}`} value={option.value} className="mt-0.5" />
          <FieldContent className="gap-1">
            <FieldLabel htmlFor={`${id}-${option.value}`} className="font-normal">
              {option.title}
            </FieldLabel>
            {option.description && <FieldDescription className="text-xs">{option.description}</FieldDescription>}
          </FieldContent>
        </Field>
      ))}
    </RadioGroup>
  )
}
