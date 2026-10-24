import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Activity, FileKey, GitBranch, Layers, Settings, SlidersHorizontal, type LucideIcon } from 'lucide-react'
import { readQuery } from '@/api/queries'
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
import { ConnectionFooter } from './connection-footer'
import { VersionedLogo } from './logo'
import { RepoPicker } from './repo-picker'

// The screens that are not built yet show where they will be, muted and out of reach.
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

export function AppSidebar(props: { repo?: string }) {
  return (
    <Sidebar>
      <SidebarHeader className="gap-6 px-4 pt-5 pb-1">
        <VersionedLogo />
        <RepoPicker current={props.repo} />
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Worktrees</SidebarGroupLabel>
          <SidebarMenu>
            <SidebarMenuItem>
              {props.repo ? (
                <SidebarMenuButton asChild isActive>
                  <Link to="/r/$repo/worktrees" params={{ repo: props.repo }}>
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
              {props.repo && <WorktreeCount repo={props.repo} />}
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
      <ConnectionFooter repo={props.repo} />
    </Sidebar>
  )
}

function WorktreeCount({ repo }: { repo: string }) {
  const list = useQuery(readQuery(repo, 'list'))
  return list.data?.items && <SidebarMenuBadge>{list.data.items.length}</SidebarMenuBadge>
}
