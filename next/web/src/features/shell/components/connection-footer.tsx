import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { cva } from 'class-variance-authority'
import { RefreshCw } from 'lucide-react'
import { fetchOrigin as fetchOriginOf, githubKey, repoKey } from '@/api/queries/commands'
import { type Connection, connectionOf, eventsQuery } from '@/api/queries/events'
import { Button } from '@/components/ui/button'
import { SidebarFooter } from '@/components/ui/sidebar'
import { cn } from '@/lib/utils'

const dot = cva('size-2 rounded-full', {
  variants: {
    connection: {
      connecting: 'bg-muted-foreground',
      live: 'bg-green-500',
      reconnecting: 'bg-amber-500',
      disconnected: 'bg-red-500',
    } satisfies Record<Connection, string>,
  },
})

const label: Record<Connection, string> = {
  connecting: 'Connecting…',
  live: 'Live',
  reconnecting: 'Reconnecting…',
  disconnected: 'Disconnected',
}

const detail: Record<Connection, string> = {
  connecting: 'Joining the event stream',
  live: 'Changes show here as they happen',
  reconnecting: 'The server stopped answering. Trying again',
  disconnected: 'The server restarted under a new session',
}

export function ConnectionFooter({ repo }: { repo: string | undefined }) {
  const client = useQueryClient()
  const events = useQuery(eventsQuery)
  const connection = connectionOf(events)
  // origin fetched, the reads on screen are read again, those asking GitHub with it: one per view, and an open
  // detail's; one off screen asks when it shows again
  const fetchOrigin = useMutation({
    mutationFn: fetchOriginOf,
    onSettled: (_data, _error, repo) =>
      Promise.all([
        client.invalidateQueries({ queryKey: repoKey(repo) }),
        client.invalidateQueries({ queryKey: githubKey(repo) }),
      ]),
  })
  const problem = fetchOrigin.error?.message ?? fetchOrigin.data?.warnings[0]?.message
  const live = connection === 'live'

  return (
    <SidebarFooter className="gap-4 border-t border-sidebar-border px-4 pt-4 pb-3">
      <div className="flex flex-col gap-0.5" role="status" aria-live="polite">
        <span className="flex items-center gap-2 text-sm font-medium">
          <span className={dot({ connection })} aria-hidden="true" />
          {label[connection]}
        </span>
        <span className={cn('text-xs text-muted-foreground', live && problem && 'text-destructive')}>
          {(live && (problem ?? events.data?.warning)) || detail[connection]}
        </span>
      </div>
      {repo && (
        <Button
          variant="outline"
          className="w-full justify-start"
          disabled={!live || fetchOrigin.isPending}
          onClick={() => fetchOrigin.mutate(repo)}
        >
          <RefreshCw className={cn(fetchOrigin.isPending && 'animate-spin')} />
          {fetchOrigin.isPending ? 'Fetching origin…' : 'Fetch origin'}
        </Button>
      )}
    </SidebarFooter>
  )
}
