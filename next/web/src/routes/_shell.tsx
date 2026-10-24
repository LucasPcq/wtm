import { Outlet, createFileRoute, useParams } from '@tanstack/react-router'
import { Unplug } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { connectionOf, eventsQuery } from '@/api/events'
import { AppSidebar } from '@/components/app-sidebar'
import { Centered } from '@/components/centered'
import { StatePage } from '@/components/state-page'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'

export const Route = createFileRoute('/_shell')({
  component: Shell,
})

// Shell is the frame of every screen: the sidebar, and the screen of the route beside it.
function Shell() {
  const { repo } = useParams({ strict: false })
  const connection = connectionOf(useQuery(eventsQuery))

  return (
    <SidebarProvider>
      <AppSidebar repo={repo} />
      <SidebarInset>
        {connection === 'disconnected' ? (
          <Centered>
            <StatePage icon={<Unplug />} title="The server restarted" command="wtm-next ui">
              It runs under a new session now. Run this in a repository to open it again; this tab picks the new session
              up on its own.
            </StatePage>
          </Centered>
        ) : (
          <Outlet />
        )}
      </SidebarInset>
    </SidebarProvider>
  )
}
