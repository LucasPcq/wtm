import { useQuery } from '@tanstack/react-query'
import { Copy } from 'lucide-react'
import { commandLineQuery } from '@/api/queries/command-line'
import type { DomainFollowUp } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { copy } from '@/features/worktrees/utils/copy'

// the fix in one click, without unfolding the row that shows it
export function CopyFollowUp({ followUp }: { followUp: DomainFollowUp }) {
  const line = useQuery(commandLineQuery(followUp))
  const text = line.data

  return (
    <Button
      variant="ghost"
      size="icon-sm"
      disabled={!text}
      aria-label={text ? `Copy ${text}` : 'Copy the command'}
      onClick={() => text && copy(text)}
    >
      <Copy />
    </Button>
  )
}
