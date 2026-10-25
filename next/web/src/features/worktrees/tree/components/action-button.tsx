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
import { CommandLine } from '@/features/worktrees/components/command-line'
import { sentence } from '@/features/worktrees/utils/sentence'

type TreeCommand = 'go' | 'sync' | 'clean'

const label: Record<TreeCommand, string> = { go: 'Go to the worktree', sync: 'Sync', clean: 'Clean' }

export function ActionButton({ action }: { action: DomainFollowUp }) {
  const isTreeCommand = (command: string | undefined): command is TreeCommand =>
    command !== undefined && Object.hasOwn(label, command)

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm" className="data-[state=open]:bg-accent">
          {isTreeCommand(action.command) ? label[action.command] : sentence(action.message)}
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
