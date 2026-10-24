import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { connectionOf, eventsQuery, type Connection } from '@/api/events'
import { json } from '@/api/fetch'
import { Button } from '@/components/ui/button'
import { SidebarFooter } from '@/components/ui/sidebar'
import { cn } from '@/lib/utils'

const looks: Record<Connection, { dot: string; label: string; detail: string }> = {
  connecting: { dot: 'bg-muted-foreground', label: 'Connecting…', detail: 'Joining the event stream' },
  live: { dot: 'bg-green-500', label: 'Live', detail: 'Events stream in from the daemon' },
  reconnecting: { dot: 'bg-amber-500', label: 'Reconnecting…', detail: 'The server stopped answering. Trying again' },
  disconnected: { dot: 'bg-red-500', label: 'Disconnected', detail: 'The server restarted under a new session' },
}

// ConnectionFooter says first whether the page is live, then offers the fetch of origin.
export function ConnectionFooter(props: { repo?: string }) {
  const events = useQuery(eventsQuery)
  const connection = connectionOf(events)
  const look = looks[connection]

  return (
    <SidebarFooter className="gap-4 border-t border-sidebar-border px-4 pt-4 pb-3">
      <div className="flex flex-col gap-0.5" role="status" aria-live="polite">
        <span className="flex items-center gap-2 text-sm font-medium">
          <span className={cn('size-2 rounded-full', look.dot)} aria-hidden="true" />
          {look.label}
        </span>
        <span className="text-xs text-muted-foreground">{(connection === 'live' && events.data?.warning) || look.detail}</span>
      </div>
      {props.repo && <FetchButton repo={props.repo} live={connection === 'live'} />}
    </SidebarFooter>
  )
}

// FetchButton runs list with its fetch answer, as the picker's refresh does on the command line.
function FetchButton({ repo, live }: { repo: string; live: boolean }) {
  const client = useQueryClient()
  const fetchOrigin = useMutation({
    mutationFn: () => json(`/api/repos/${repo}/commands/list`, { answers: [{ path: 'fetch', values: ['true'] }] }),
    onSettled: () => client.invalidateQueries({ queryKey: ['repo', repo] }),
  })

  return (
    <Button variant="outline" className="w-full justify-start" disabled={!live || fetchOrigin.isPending} onClick={() => fetchOrigin.mutate()}>
      <RefreshCw className={cn(fetchOrigin.isPending && 'animate-spin')} />
      {fetchOrigin.isPending ? 'Fetching origin…' : 'Fetch origin'}
    </Button>
  )
}
