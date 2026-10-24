import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Check, ChevronsUpDown } from 'lucide-react'
import { useState } from 'react'
import { reposQuery } from '@/api/queries/repos'
import { Command, CommandEmpty, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { repoName } from '@/lib/repo'

export function RepoPicker(props: { current: string | undefined }) {
  const repos = useQuery(reposQuery)
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const current = repos.data?.find((repo) => repo.id === props.current)

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex w-full items-center gap-2 rounded-md border border-sidebar-border bg-sidebar-accent px-2 py-1.5 text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <span className="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted text-[13px] font-semibold uppercase">
            {current ? repoName(current.root).slice(0, 1) : '?'}
          </span>
          <span className="flex min-w-0 flex-1 flex-col">
            <span className="truncate text-sm font-medium">
              {current ? repoName(current.root) : 'Choose a repository'}
            </span>
            <span className="truncate text-xs text-muted-foreground">
              {current ? current.root : `${repos.data?.length ?? 0} known`}
            </span>
          </span>
          <ChevronsUpDown className="size-4 shrink-0 text-muted-foreground" />
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-(--radix-popover-trigger-width) p-0">
        <Command>
          <CommandInput placeholder="Search…" />
          <CommandList>
            <CommandEmpty>No repository matches.</CommandEmpty>
            {repos.data?.map((repo) => (
              <CommandItem
                key={repo.id}
                value={repo.root}
                onSelect={() => {
                  setOpen(false)
                  void navigate({ to: '/r/$repo/worktrees', params: { repo: repo.id } })
                }}
              >
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="truncate">{repoName(repo.root)}</span>
                  <span className="truncate text-xs text-muted-foreground">{repo.root}</span>
                </span>
                {repo.id === props.current && <Check className="size-4" />}
              </CommandItem>
            ))}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
