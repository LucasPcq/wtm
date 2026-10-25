import { Copy } from 'lucide-react'
import { copy } from '@/features/worktrees/shared/copy'
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

export function CopyPath({ path }: { path: string }) {
  return (
    <button
      type="button"
      className="flex max-w-full min-w-0 items-center gap-1.5 text-muted-foreground hover:text-foreground"
      onClick={() => copy(path)}
    >
      <Path path={path} className="text-inherit" />
      <Copy className="size-3 shrink-0" aria-label="Copy the path" />
    </button>
  )
}
