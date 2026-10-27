import { useQuery } from '@tanstack/react-query'
import { Unplug } from 'lucide-react'
import type { ReactNode } from 'react'
import { Toaster } from 'sonner'
import { connectionOf, eventsQuery } from '@/api/queries/events'
import { Centered } from '@/components/centered'
import { StatePage } from '@/components/state-page'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { AppSidebar } from '@/features/shell/components/app-sidebar'

export function Shell({ repo, children }: { repo: string | undefined; children: ReactNode }) {
  const connection = connectionOf(useQuery(eventsQuery))

  return (
    <SidebarProvider>
      <AppSidebar repo={repo} />
      <SidebarInset className="h-svh min-h-0 overflow-hidden">
        {connection === 'disconnected' ? (
          <Centered>
            <StatePage icon={<Unplug />} title="The server restarted" command="wtm-next ui">
              It runs under a new session now. Run this in a repository to open it again; this tab picks the new session
              up on its own.
            </StatePage>
          </Centered>
        ) : (
          children
        )}
      </SidebarInset>
      {/* the glyph carries the colour: amber, something left to do */}
      <Toaster
        position="bottom-right"
        theme="system"
        toastOptions={{ classNames: { warning: '[&_[data-icon]]:text-amber-500' } }}
      />
    </SidebarProvider>
  )
}
