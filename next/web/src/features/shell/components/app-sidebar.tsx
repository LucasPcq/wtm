import { useQuery } from '@tanstack/react-query'
import { Link, useMatchRoute } from '@tanstack/react-router'
import { Activity, FileKey, GitBranch, Layers, type LucideIcon, Settings, SlidersHorizontal } from 'lucide-react'
import { jobsUp, readQuery, worktreeCount } from '@/api/queries/commands'
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuBadge,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/components/ui/sidebar'
import { VersionedLogo } from '@/components/versioned-logo'
import { ConnectionFooter } from '@/features/shell/components/connection-footer'
import { RepoPicker } from '@/features/shell/components/repo-picker'

const soon: { group: string; items: { label: string; icon: LucideIcon }[] }[] = [
  {
    group: 'Run',
    items: [
      { label: 'Profiles & jobs', icon: Layers },
      { label: '.env files', icon: FileKey },
      { label: 'Run settings', icon: SlidersHorizontal },
    ],
  },
  { group: 'Project', items: [{ label: 'Settings', icon: Settings }] },
]

export function AppSidebar({ repo }: { repo: string | undefined }) {
  const matchRoute = useMatchRoute()
  const onRunning = matchRoute({ to: '/r/$repo/running', fuzzy: true }) !== false

  return (
    <Sidebar>
      <SidebarHeader className="gap-6 pt-5">
        <div className="px-2">
          <VersionedLogo />
        </div>
        <RepoPicker current={repo} />
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Worktrees</SidebarGroupLabel>
          <SidebarMenu>
            <SidebarMenuItem>
              {repo ? (
                <SidebarMenuButton asChild isActive={!onRunning}>
                  <Link to="/r/$repo/worktrees" params={{ repo: repo }}>
                    <GitBranch />
                    All worktrees
                  </Link>
                </SidebarMenuButton>
              ) : (
                <SidebarMenuButton disabled>
                  <GitBranch />
                  All worktrees
                </SidebarMenuButton>
              )}
              {repo && <WorktreeCount repo={repo} />}
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroup>
        {soon.map(({ group, items }) => (
          <SidebarGroup key={group}>
            <SidebarGroupLabel>{group}</SidebarGroupLabel>
            <SidebarMenu>
              {group === 'Run' && (
                <SidebarMenuItem>
                  {repo ? (
                    <SidebarMenuButton asChild isActive={onRunning}>
                      <Link to="/r/$repo/running" params={{ repo }}>
                        <Activity />
                        Running
                      </Link>
                    </SidebarMenuButton>
                  ) : (
                    <SidebarMenuButton disabled>
                      <Activity />
                      Running
                    </SidebarMenuButton>
                  )}
                  {repo && <JobsUp repo={repo} />}
                </SidebarMenuItem>
              )}
              {items.map(({ label, icon: Icon }) => (
                <SidebarMenuItem key={label}>
                  <SidebarMenuButton disabled>
                    <Icon />
                    {label}
                  </SidebarMenuButton>
                  <SidebarMenuBadge className="text-muted-foreground">soon</SidebarMenuBadge>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroup>
        ))}
      </SidebarContent>
      <ConnectionFooter repo={repo} />
    </Sidebar>
  )
}

// it reads nothing of its own: it counts what the view on screen read last, so that the tree is not paid with a list
// read on every event
function WorktreeCount({ repo }: { repo: string }) {
  const list = useQuery({ ...readQuery(repo, 'list'), enabled: false })
  const tree = useQuery({ ...readQuery(repo, 'tree'), enabled: false })

  const read = tree.dataUpdatedAt > list.dataUpdatedAt ? tree.data : list.data
  return read && <SidebarMenuBadge>{worktreeCount(read)}</SidebarMenuBadge>
}

// shortcut: as WorktreeCount, it counts what list's read said last; on the tree view that read is off and the count
// waits for it, until the jobs read (LUC-345) gives the page and this badge their own
function JobsUp({ repo }: { repo: string }) {
  const list = useQuery({ ...readQuery(repo, 'list'), enabled: false })
  const up = list.data ? jobsUp(list.data) : 0
  return up > 0 && <SidebarMenuBadge>{up}</SidebarMenuBadge>
}
