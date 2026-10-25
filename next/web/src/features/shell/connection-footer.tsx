import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { cva } from 'class-variance-authority'
import { RefreshCw } from 'lucide-react'
import { connectionOf, eventsQuery, type Connection } from '@/api/queries/events'
import { githubKey, githubQuery, refreshList, repoKey } from '@/api/queries/commands'
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

// ConnectionFooter says first whether the page is live, then offers the fetch of origin and says how it went
// wrong, if it did.
export function ConnectionFooter({ repo }: { repo: string | undefined }) {
  const client = useQueryClient()
  const events = useQuery(eventsQuery)
  const connection = connectionOf(events)
  // list fetching origin and asking GitHub, as the picker's refresh does: GitHub's list is the one it read, its
  // other reads (an open detail's) are asked again
  const fetchOrigin = useMutation({
    mutationFn: refreshList,
    onSuccess: (read, repo) => {
      client.setQueryData(githubQuery(repo, 'list').queryKey, read)
      void client.invalidateQueries({ queryKey: githubKey(repo), predicate: (q) => q.queryKey[2] !== 'list' })
    },
    onSettled: (_data, _error, repo) => client.invalidateQueries({ queryKey: repoKey(repo) }),
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
