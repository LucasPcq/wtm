import { cva } from 'class-variance-authority'
import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

export type Tone = 'ok' | 'attention' | 'failed' | 'muted' | 'busy'

const icon = cva('size-3.5 shrink-0', {
  variants: {
    tone: {
      ok: 'text-green-500',
      attention: 'text-amber-500',
      failed: 'text-destructive',
      muted: 'text-muted-foreground',
      busy: 'animate-spin text-muted-foreground motion-reduce:animate-none',
    } satisfies Record<Tone, string>,
  },
})

export function Glyph({
  icon: Icon,
  tone,
  children,
  className,
}: {
  icon: LucideIcon
  tone: Tone
  children: ReactNode
  className?: string
}) {
  return (
    <span
      className={cn('inline-flex min-w-0 items-center gap-1.5', tone === 'busy' && 'text-muted-foreground', className)}
    >
      <Icon className={icon({ tone })} aria-hidden="true" />
      <span className="truncate">{children}</span>
    </span>
  )
}
