import { GitPullRequest } from 'lucide-react'
import type { DomainOption, WorktreePullRequest } from '@/api/types.gen'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { CopyCommand } from '@/features/worktrees/components/copy-command'
import { optionLine } from '@/features/worktrees/create/utils/checkout-words'
import { cn } from '@/lib/utils'

// its icon slot lines its text up with the search's; a pull request a problem keeps from being chosen is greyed from the start, its command to copy beside it
export function PrRow({
  option,
  pr,
  chosen,
  onChoose,
}: {
  option: DomainOption
  pr: WorktreePullRequest
  chosen: boolean
  onChoose: () => void
}) {
  const command = option.problem?.params?.command
  const followUp = option.problem?.follow_up

  return (
    <li className="flex items-center gap-2">
      <button
        type="button"
        aria-pressed={chosen}
        disabled={option.problem !== undefined}
        onClick={onChoose}
        className={cn(
          'flex min-w-0 flex-1 items-start gap-3 rounded-md px-2.5 py-2 text-left outline-none',
          'hover:bg-muted focus-visible:ring-[3px] focus-visible:ring-ring/50 aria-pressed:bg-muted',
          'disabled:pointer-events-none disabled:text-muted-foreground',
        )}
      >
        <GitPullRequest className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="truncate text-sm">
            #{pr.number} {pr.title}
          </span>
          <span className="truncate text-xs text-muted-foreground">{optionLine(option)}</span>
        </span>
      </button>
      {command && <CopyCommand text={command} look="copy" />}
      {!command && followUp && <CommandLine followUp={followUp} look="copy" />}
    </li>
  )
}
