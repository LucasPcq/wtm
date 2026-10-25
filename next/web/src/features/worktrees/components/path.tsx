import { cn } from '@/lib/utils'

export function Path({ path, className }: { path: string; className?: string }) {
  return (
    <span
      title={path}
      className={cn('truncate text-left font-mono text-xs text-muted-foreground [direction:rtl]', className)}
    >
      <bdi>{path}</bdi>
    </span>
  )
}
