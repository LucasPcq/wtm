import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import type { ContextMenuItem } from '@/components/ui/context-menu'
import type { DropdownMenuItem } from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'

// aria-disabled, not disabled: the keyboard skips a Radix disabled item, so its reason would be out of reach
export function MenuEntry({
  Item,
  icon: Icon,
  label,
  why,
  destructive = false,
  onSelect,
  children,
}: {
  Item: typeof ContextMenuItem | typeof DropdownMenuItem
  icon: LucideIcon
  label: string
  why: string | undefined
  destructive?: boolean
  onSelect?: () => void
  children?: ReactNode
}) {
  return (
    <Item
      aria-disabled={why !== undefined || undefined}
      variant={destructive ? 'destructive' : 'default'}
      className={cn('items-start [&>svg]:mt-0.5', why !== undefined && 'opacity-50')}
      onSelect={(event) => {
        if (why !== undefined) return event.preventDefault()
        onSelect?.()
      }}
    >
      <Icon />
      <span className="flex min-w-0 flex-1 flex-col">
        {label}
        {why && <span className="text-xs text-muted-foreground">{why}</span>}
      </span>
      {children}
    </Item>
  )
}
