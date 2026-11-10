import { Pencil } from 'lucide-react'
import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'

export function ValueRow({
  label,
  help,
  value,
  isDefault,
  onChange,
}: {
  label: string
  help: ReactNode
  value: ReactNode
  isDefault: boolean
  onChange: () => void
}) {
  return (
    <div className="flex items-start justify-between gap-6">
      <div className="flex min-w-0 flex-col gap-1">
        <span className="text-sm font-medium">{label}</span>
        <span className="text-sm text-muted-foreground">{help}</span>
      </div>
      <Button
        variant="outline"
        className="h-10 w-80 shrink-0 justify-between gap-2 font-normal"
        aria-label={`Change ${label}`}
        onClick={onChange}
      >
        <span className="truncate font-mono text-sm">{value}</span>
        <span className="flex items-center gap-2 text-xs text-muted-foreground">
          {isDefault && 'default'}
          <Pencil />
        </span>
      </Button>
    </div>
  )
}
