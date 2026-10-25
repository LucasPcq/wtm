import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { ChevronsUpDown } from 'lucide-react'
import { useState } from 'react'
import { reposQuery } from '@/api/queries/repos'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem } from '@/components/ui/sidebar'
import { repoName } from '@/lib/repo'

export function RepoPicker({ current: id }: { current: string | undefined }) {
  const repos = useQuery(reposQuery)
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const current = repos.data?.find((repo) => repo.id === id)

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <Popover open={open} onOpenChange={setOpen}>
          <PopoverTrigger asChild>
            <SidebarMenuButton size="lg" className="border border-sidebar-border bg-sidebar-accent">
              <span className="flex size-8 shrink-0 items-center justify-center rounded-md bg-muted text-[13px] font-semibold uppercase">
                {current ? repoName(current.root).slice(0, 1) : '?'}
              </span>
              <span className="flex min-w-0 flex-1 flex-col">
                <span className="truncate font-medium">{current ? repoName(current.root) : 'Choose a repository'}</span>
                <span className="truncate text-xs text-muted-foreground">
                  {current ? current.root : `${repos.data?.length ?? 0} known`}
                </span>
              </span>
              <ChevronsUpDown className="ml-auto text-muted-foreground" />
            </SidebarMenuButton>
          </PopoverTrigger>
          <PopoverContent align="start" className="w-(--radix-popover-trigger-width) p-0">
            <Command>
              <CommandInput placeholder="Search…" />
              <CommandList>
                <CommandEmpty>No repository matches.</CommandEmpty>
                <CommandGroup>
                  {repos.data?.map((repo) => (
                    <CommandItem
                      key={repo.id}
                      value={repo.root}
                      data-checked={repo.id === id}
                      onSelect={() => {
                        setOpen(false)
                        void navigate({ to: '/r/$repo/worktrees', params: { repo: repo.id } })
                      }}
                    >
                      <span className="flex min-w-0 flex-1 flex-col">
                        <span className="truncate">{repoName(repo.root)}</span>
                        <span className="truncate text-xs text-muted-foreground">{repo.root}</span>
                      </span>
                    </CommandItem>
                  ))}
                </CommandGroup>
              </CommandList>
            </Command>
          </PopoverContent>
        </Popover>
      </SidebarMenuItem>
    </SidebarMenu>
  )
}
