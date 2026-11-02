import type { CSSProperties, ReactNode } from 'react'
import { cn } from '@/lib/utils'

// a node's children, each <li> drawing its piece of the rail and its elbow: the last one's piece stops at its elbow,
// so the rail ends on the last child over any surface, with no mask painted in a background colour
export function Rail({
  elbow,
  indent,
  className,
  children,
}: {
  elbow: number
  indent: number
  className?: string
  children: ReactNode
}) {
  return (
    <ul
      style={{ '--elbow': `${elbow}px`, '--indent': `${indent}px` } as CSSProperties}
      className={cn(
        'flex flex-col pl-(--indent) *:relative',
        '*:before:absolute *:before:inset-y-0 *:before:left-[calc(var(--indent)*-1)] *:before:border-l',
        '*:last:before:bottom-auto *:last:before:h-[calc(var(--elbow)+1px)]',
        '*:after:absolute *:after:top-(--elbow) *:after:left-[calc(var(--indent)*-1)] *:after:w-(--indent) *:after:border-t',
        className,
      )}
    >
      {children}
    </ul>
  )
}
