import type { DomainOption, WorktreePullRequest } from '@/api/types.gen'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { CopyCommand } from '@/features/worktrees/create/components/copy-command'
import { optionLine } from '@/features/worktrees/create/utils/checkout-words'
import { cn } from '@/lib/utils'

// a pull request a problem keeps from being chosen is greyed from the start, its command to copy beside it
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
          'flex min-w-0 flex-1 flex-col items-start gap-0.5 rounded-md px-2.5 py-2 text-left outline-none',
          'hover:bg-muted focus-visible:ring-[3px] focus-visible:ring-ring/50 aria-pressed:bg-muted',
          'disabled:pointer-events-none disabled:text-muted-foreground',
        )}
      >
        <span className="w-full truncate text-sm">
          #{pr.number} {pr.title}
        </span>
        <span className="w-full truncate text-xs text-muted-foreground">{optionLine(option)}</span>
      </button>
      {command && <CopyCommand text={command} look="copy" />}
      {!command && followUp && <CommandLine followUp={followUp} look="copy" />}
    </li>
  )
}
