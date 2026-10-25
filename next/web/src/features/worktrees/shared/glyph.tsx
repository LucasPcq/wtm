import { cva } from 'class-variance-authority'
import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

// A Tone is what a state means, the only thing its colour says: healthy, needs the person, failed, nothing to say,
// under way.
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

// Glyph is a state as an icon of its tone and the word that says it: colour never carries the state alone.
export function Glyph(props: { icon: LucideIcon; tone: Tone; children: ReactNode; className?: string }) {
  return (
    <span className={cn('inline-flex min-w-0 items-center gap-1.5', props.className)}>
      <props.icon className={icon({ tone: props.tone })} aria-hidden="true" />
      <span className="truncate">{props.children}</span>
    </span>
  )
}
