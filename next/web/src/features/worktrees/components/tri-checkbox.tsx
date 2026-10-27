import { Minus } from 'lucide-react'
import { Checkbox } from '@/components/ui/checkbox'

// checked, unchecked, or some: shadcn's checkbox draws its check when only some are checked, so a minus stands in
export function TriCheckbox({
  label,
  checked,
  disabled,
  onToggle,
}: {
  label: string
  checked: boolean | 'indeterminate'
  disabled: boolean
  onToggle: () => void
}) {
  return (
    <span className="relative flex">
      <Checkbox
        aria-label={label}
        checked={checked}
        disabled={disabled}
        onCheckedChange={onToggle}
        className="data-[state=indeterminate]:border-primary data-[state=indeterminate]:bg-primary dark:data-[state=indeterminate]:bg-primary [&[data-state=indeterminate]_svg]:hidden"
      />
      {checked === 'indeterminate' && (
        <Minus
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 m-auto size-3.5 text-primary-foreground"
        />
      )}
    </span>
  )
}
