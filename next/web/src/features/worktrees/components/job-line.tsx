import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

// one job on a run's Confirm: its mark (an order, a glyph), its name, what the run does to it
export function JobLine({
  icon,
  name,
  mono = false,
  children,
}: {
  icon: ReactNode
  name: string
  mono?: boolean
  children: ReactNode
}) {
  return (
    <li aria-label={name} className="flex items-center gap-3 text-sm">
      <span className="flex size-5 shrink-0 items-center justify-center">{icon}</span>
      <span className="font-medium">{name}</span>
      <span className={cn('min-w-0 truncate text-muted-foreground', mono && 'font-mono text-xs')}>{children}</span>
    </li>
  )
}
