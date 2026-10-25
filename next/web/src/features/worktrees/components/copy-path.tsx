import { Copy } from 'lucide-react'
import { Path } from '@/features/worktrees/components/path'
import { copy } from '@/features/worktrees/utils/copy'

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
