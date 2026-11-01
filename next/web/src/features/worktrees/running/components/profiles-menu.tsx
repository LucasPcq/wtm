import { useQuery } from '@tanstack/react-query'
import { Layers, Play } from 'lucide-react'
import { questionsQuery } from '@/api/queries/commands'
import {
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from '@/components/ui/dropdown-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { openRunUp } from '@/features/worktrees/hooks/use-run-up-dialog'
import { jobsSummary } from '@/features/worktrees/run-up/utils/words'

// a menu shows five: past them the dialog, which has the search, lists them all
const shown = 5

export function ProfilesMenu({ repo, worktree }: { repo: string; worktree: string }) {
  const form = useQuery(questionsQuery(repo, 'run-up', [{ path: 'worktree', values: [worktree] }]))
  const field = form.data?.fields.find((one) => one.path === 'profile')
  const profiles = field?.kind === 'select' ? field.options : []
  const ordered = [...profiles.filter((one) => one.default), ...profiles.filter((one) => !one.default)]

  return (
    <DropdownMenuSub>
      <DropdownMenuSubTrigger>
        <Play />
        Start a profile
      </DropdownMenuSubTrigger>
      <DropdownMenuSubContent className="w-72">
        <DropdownMenuLabel className="text-xs text-muted-foreground">
          Profiles{form.data && ` · ${profiles.length}`}
        </DropdownMenuLabel>
        {!form.data && <Skeleton className="m-2 h-8" />}
        {ordered.slice(0, shown).map((profile) => {
          const running = profile.running_in?.includes(worktree) ?? false
          return (
            <DropdownMenuItem
              key={profile.value}
              disabled={running}
              className="items-start [&>svg]:mt-0.5"
              onSelect={() => openRunUp(repo, { worktrees: [worktree], profile: profile.value })}
            >
              <Play />
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="font-medium">{profile.value}</span>
                <span className="text-xs text-muted-foreground">{jobsSummary(profile.jobs)}</span>
              </span>
              {running && <span className="text-xs text-muted-foreground">running</span>}
              {!running && profile.default && <span className="text-xs text-muted-foreground">default</span>}
            </DropdownMenuItem>
          )
        })}
        {profiles.length > shown && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              className="items-start [&>svg]:mt-0.5"
              onSelect={() => openRunUp(repo, { worktrees: [worktree], step: 'profile' })}
            >
              <Layers />
              <span className="flex flex-col gap-0.5">
                All {profiles.length} profiles…
                <span className="text-xs text-muted-foreground">opens Start a profile</span>
              </span>
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuSubContent>
    </DropdownMenuSub>
  )
}
