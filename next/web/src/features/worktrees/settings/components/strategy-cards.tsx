import type { DomainSelectField } from '@/api/types.gen'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { type Strategy, strategyWords } from '@/features/worktrees/settings/utils/words'
import { cn } from '@/lib/utils'

const isStrategy = (value: string): value is Strategy => value in strategyWords

export function StrategyCards({
  field,
  defaultValue,
  disabled,
  onChoose,
}: {
  field: DomainSelectField
  defaultValue: string | undefined
  disabled: boolean
  onChoose: (value: Strategy) => void
}) {
  return (
    <RadioGroup
      aria-label="Where .env files come from"
      value={field.value}
      disabled={disabled}
      onValueChange={(value) => isStrategy(value) && onChoose(value)}
      className="grid grid-cols-2 gap-3 xl:grid-cols-4"
    >
      {field.options.flatMap(({ value }) => {
        if (!isStrategy(value)) return []
        const words = strategyWords[value]
        const id = `strategy-${value}`

        return (
          <label
            key={value}
            htmlFor={id}
            className={cn(
              'flex cursor-pointer flex-col gap-1 rounded-lg border p-4 transition-colors hover:bg-muted/40 has-disabled:cursor-default has-disabled:opacity-60',
              value === field.value && 'border-foreground',
            )}
          >
            <span className="flex items-center gap-2 text-sm font-medium">
              <RadioGroupItem id={id} value={value} />
              {words.title}
              {value === defaultValue && <span className="text-xs font-normal text-muted-foreground">default</span>}
            </span>
            <span className="pl-6 font-mono text-xs">{words.example}</span>
            <span className="pl-6 text-xs text-muted-foreground">{words.line}</span>
          </label>
        )
      })}
    </RadioGroup>
  )
}
