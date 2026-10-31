import { Copy } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { copy } from '@/features/worktrees/utils/copy'

// a command of another tool (gh), which the server does not spell: copied as its text, in CommandLine's looks
export function CopyCommand({ text, look = 'block' }: { text: string; look?: 'block' | 'inline' | 'copy' }) {
  const inline = look === 'inline'
  const button = (
    <Button
      variant="ghost"
      size={inline ? 'icon-xs' : 'icon-sm'}
      aria-label={`Copy ${text}`}
      onClick={() => copy(text)}
    >
      <Copy />
    </Button>
  )

  if (look === 'copy') return button

  if (inline)
    return (
      <span className="flex shrink-0 items-center gap-0.5 rounded-md bg-muted py-0.5 pr-0.5 pl-2">
        <code className="font-mono text-xs">{text}</code>
        {button}
      </span>
    )

  return (
    <div className="flex items-center gap-2 rounded-md bg-muted py-1 pr-1 pl-3">
      <code className="min-w-0 flex-1 font-mono text-[13px] break-all">{text}</code>
      {button}
    </div>
  )
}
