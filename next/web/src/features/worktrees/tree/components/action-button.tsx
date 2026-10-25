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
import { CommandLine } from '@/features/worktrees/shared/components/command-line'
import { sentence } from '@/features/worktrees/shared/utils/sentence'

// the commands the tree offers as its action of the moment; another one is named by what it does
const label: Record<string, string> = { go: 'Go to the worktree', sync: 'Sync', clean: 'Clean' }

export function ActionButton({ action }: { action: DomainFollowUp }) {
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm" className="data-[state=open]:bg-accent">
          {label[action.command ?? ''] ?? sentence(action.message)}
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
