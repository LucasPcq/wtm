import { type ReactNode, useId } from 'react'
import { Checkbox } from '@/components/ui/checkbox'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { cn } from '@/lib/utils'

export type Choice = { id: string; label: string; hint?: string | undefined }

export function ChoicePopover({
  trigger,
  head,
  groups,
  shown,
  onToggle,
  side,
  className,
}: {
  trigger: ReactNode
  head?: ReactNode
  groups: { title?: string; choices: Choice[] }[]
  shown: readonly string[]
  onToggle: (id: string) => void
  side?: 'top' | 'bottom'
  className?: string
}) {
  const prefix = useId()

  return (
    <Popover>
      <PopoverTrigger asChild>{trigger}</PopoverTrigger>
      <PopoverContent align={side === 'top' ? 'start' : 'end'} side={side} className={cn('p-0', className)}>
        {head}
        <div className={cn('grid gap-x-3 gap-y-2 p-2', groups.length > 1 && 'grid-cols-2 pt-1 pb-3')}>
          {groups.map(({ title, choices }) => (
            <fieldset key={title ?? 'choices'} className="flex flex-col">
              {title && <legend className="px-2 py-1 text-xs font-medium text-muted-foreground">{title}</legend>}
              {choices.map(({ id, label, hint }) => (
                <label
                  key={id}
                  htmlFor={`${prefix}-${id}`}
                  className="flex cursor-pointer items-start gap-2.5 rounded-md px-2 py-1.5 hover:bg-accent"
                >
                  <Checkbox
                    id={`${prefix}-${id}`}
                    className="mt-0.5"
                    checked={shown.includes(id)}
                    onCheckedChange={() => onToggle(id)}
                  />
                  <span className="flex flex-col">
                    <span className={cn('text-sm', hint && 'font-medium')}>{label}</span>
                    {hint && <span className="text-xs text-muted-foreground">{hint}</span>}
                  </span>
                </label>
              ))}
            </fieldset>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  )
}
