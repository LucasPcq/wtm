import { useQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { questionsQuery } from '@/api/queries/commands'
import { DropdownMenuItem } from '@/components/ui/dropdown-menu'
import { MenuEntry } from '@/features/worktrees/components/menu-entry'
import { openRunStart } from '@/features/worktrees/hooks/use-run-start-dialog'
import { plural } from '@/lib/plural'

// the declared jobs not running in the worktree, as run start offers them
export function StartJobEntry({ repo, worktree }: { repo: string; worktree: string }) {
  const form = useQuery(questionsQuery(repo, 'run-start', [{ path: 'worktree', values: [worktree] }]))
  const field = form.data?.fields.find((one) => one.path === 'job')
  const idle = (field?.kind === 'select' ? field.options : []).filter((option) => !option.running_in?.length)

  const why = () => {
    if (!form.data) return 'Reading the jobs…'
    return idle.length === 0 ? 'Every declared job runs already' : undefined
  }

  return (
    <MenuEntry
      Item={DropdownMenuItem}
      icon={Plus}
      label="Start one job…"
      description={`choose among the ${plural(idle.length, 'job')} not running`}
      why={why()}
      onSelect={() => openRunStart(repo, { worktree })}
    />
  )
}
