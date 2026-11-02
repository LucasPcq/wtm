import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

// a progress row carries its own padding: its list packs tighter than Confirm's lines
export function JobSection({
  title,
  dense = false,
  children,
}: {
  title: string
  dense?: boolean
  children: ReactNode
}) {
  return (
    <section aria-label={title} className="flex flex-col gap-2">
      <h3 className="text-xs font-medium text-muted-foreground">{title}</h3>
      <ul className={cn('flex flex-col', dense ? 'gap-1' : 'gap-2')}>{children}</ul>
    </section>
  )
}
