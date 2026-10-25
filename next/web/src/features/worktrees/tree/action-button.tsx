import type { DomainFollowUp } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { CommandLine } from '@/features/worktrees/shared/command-line'
import { sentence } from '@/features/worktrees/shared/sentence'

export function ActionButton({ action }: { action: DomainFollowUp }) {
  const name = () => {
    if (action.command === 'go') return 'Go to the worktree'
    const command = action.command ?? 'open'
    return command.charAt(0).toUpperCase() + command.slice(1)
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm" className="data-[state=open]:bg-accent">
          {name()}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="flex w-96 flex-col gap-3">
        <PopoverHeader>
          <PopoverTitle>{sentence(action.message)}</PopoverTitle>
          <PopoverDescription>Not in the web interface yet: copy this into a terminal.</PopoverDescription>
        </PopoverHeader>
        <CommandLine followUp={action} />
      </PopoverContent>
    </Popover>
  )
}
