import { useId } from 'react'
import { Switch } from '@/components/ui/switch'

export function OptionSwitch({
  title,
  line,
  on,
  onSwitch,
}: {
  title: string
  line: string
  on: boolean
  onSwitch: (on: boolean) => void
}) {
  const id = useId()

  return (
    <div className="flex items-start gap-4 px-4 py-3">
      <Switch id={id} checked={on} onCheckedChange={onSwitch} className="mt-0.5" />
      <span className="flex min-w-0 flex-col gap-1">
        <label htmlFor={id} className="cursor-pointer text-sm font-medium">
          {title}
        </label>
        {line && <span className="text-xs text-muted-foreground">{line}</span>}
      </span>
    </div>
  )
}
