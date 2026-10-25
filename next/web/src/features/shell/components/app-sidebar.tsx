import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Activity, FileKey, GitBranch, Layers, type LucideIcon, Settings, SlidersHorizontal } from 'lucide-react'
import { readQuery, worktreeCount } from '@/api/queries/commands'
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
      { label: 'Running', icon: Activity },
      { label: 'Profiles & jobs', icon: Layers },
      { label: '.env files', icon: FileKey },
      { label: 'Run settings', icon: SlidersHorizontal },
    ],
  },
  { group: 'Project', items: [{ label: 'Settings', icon: Settings }] },
]

export function AppSidebar({ repo }: { repo: string | undefined }) {
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
                <SidebarMenuButton asChild isActive>
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
