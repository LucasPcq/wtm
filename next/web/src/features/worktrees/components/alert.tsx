import { CircleX, type LucideIcon, TriangleAlert } from 'lucide-react'
import type { ComponentProps, ReactNode } from 'react'
import { AlertDescription, AlertTitle, Alert as Box } from '@/components/ui/alert'
import { cn } from '@/lib/utils'

export type AlertTone = 'amber' | 'red' | 'muted'

const look: Record<AlertTone, { box: string; icon: string; glyph: LucideIcon }> = {
  amber: { box: 'border-amber-500/35 bg-amber-500/8', icon: 'text-amber-500', glyph: TriangleAlert },
  red: { box: 'border-destructive/35 bg-destructive/8', icon: 'text-destructive', glyph: CircleX },
  muted: { box: 'border-border bg-muted/40', icon: 'text-muted-foreground', glyph: TriangleAlert },
}

// shadcn's grid: the icon top-aligned with the title, title and description in column 2, a compact action on the
// title line; what goes below (a full-width fix and its consequence, a long command) spans the whole width
export function Alert({
  tone,
  icon,
  title,
  description,
  action,
  below,
  className,
  ...props
}: {
  tone: AlertTone
  icon?: LucideIcon
  title?: ReactNode
  description?: ReactNode
  action?: ReactNode
  below?: ReactNode
} & Omit<ComponentProps<'div'>, 'title'>) {
  const { box, icon: color, glyph } = look[tone]
  const Icon = icon ?? glyph

  return (
    <Box
      className={cn(
        'gap-y-1 px-4 py-3 text-foreground has-[>svg]:grid-cols-[auto_1fr_auto] has-[>svg]:gap-x-3',
        box,
        className,
      )}
      {...props}
    >
      <Icon className={cn('size-4', color)} aria-hidden="true" />
      {title && <AlertTitle className="col-start-2 min-w-0">{title}</AlertTitle>}
      {description && (
        <AlertDescription className={cn('col-start-2 min-w-0 text-pretty', !title && 'row-start-1')}>
          {description}
        </AlertDescription>
      )}
      {action && <div className="col-start-3 row-span-2 row-start-1 self-start">{action}</div>}
      {below && <div className="col-span-3 mt-2 flex flex-col gap-2">{below}</div>}
    </Box>
  )
}
