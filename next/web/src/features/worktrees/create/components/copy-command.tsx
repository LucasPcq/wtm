import { Copy } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { copy } from '@/features/worktrees/utils/copy'

// a command of another tool (gh), which the server does not spell: copied as its text
export function CopyCommand({ text, look = 'block' }: { text: string; look?: 'block' | 'copy' }) {
  const button = (
    <Button variant="ghost" size="icon-sm" aria-label={`Copy ${text}`} onClick={() => copy(text)}>
      <Copy />
    </Button>
  )

  if (look === 'copy') return button

  return (
    <div className="flex items-center gap-2 rounded-md bg-muted py-1 pr-1 pl-3">
      <code className="min-w-0 flex-1 font-mono text-[13px] break-all">{text}</code>
      {button}
    </div>
  )
}
