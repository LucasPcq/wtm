import { Field, FieldDescription, FieldLabel } from '@/components/ui/field'
import { Select, SelectTrigger, SelectValue } from '@/components/ui/select'
import { checkoutWords } from '@/features/worktrees/create/utils/checkout-words'

// the parent is the pull request's base: shown before one is chosen, so the step does not change shape once it is
export function ParentPlaceholder() {
  const { label, help } = checkoutWords(undefined).from

  return (
    <Field className="gap-2" data-disabled>
      <div className="flex flex-col gap-1">
        <FieldLabel htmlFor="parent-placeholder">{label}</FieldLabel>
        <FieldDescription className="text-xs text-pretty">{help}</FieldDescription>
      </div>
      <Select disabled>
        <SelectTrigger id="parent-placeholder" className="w-full">
          <SelectValue placeholder="Set by the pull request's base" />
        </SelectTrigger>
      </Select>
    </Field>
  )
}
