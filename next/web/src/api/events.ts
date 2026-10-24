import { queryOptions, experimental_streamedQuery as streamedQuery, type QueryClient } from '@tanstack/react-query'
import { EventSourceParserStream } from 'eventsource-parser/stream'
import { HttpError, request } from './fetch'
import type { WebRepos, WebWarning, WtmEventsV2 } from './types.gen'

// Live is what the event stream says of itself: what kept a line or a repository from it for a while (the bus
// lost, a repository skipped), until the next line.
export type Live = { warning?: string }

export const reconnectEvery = 2000

// eventsQuery holds the event stream of every repository open. Each line invalidates what it changed; the stream
// ending or failing is retried every reconnectEvery, until the server refuses the token: it restarted under a new
// one, which a token change brings (main.tsx invalidates every query then).
export const eventsQuery = queryOptions({
  queryKey: ['events'],
  queryFn: streamedQuery<Live, Live>({
    streamFn: ({ client, signal }) => lines(client, signal),
    refetchMode: 'append',
    reducer: (_, live) => live,
    initialValue: {},
  }),
  retry: false,
  staleTime: Infinity,
  refetchInterval: (query) => (refused(query.state.error) ? false : reconnectEvery),
  refetchIntervalInBackground: true,
})

const refused = (error: unknown) => error instanceof HttpError && error.status === 401

async function* lines(client: QueryClient, signal: AbortSignal): AsyncGenerator<Live> {
  const response = await request('/api/events', { signal })
  if (!response.ok || !response.body) throw new HttpError(response.status, 'the event stream was refused')
  // back after a break: the lines in between are lost, so everything is read again
  if (client.getQueryData(eventsQuery.queryKey)) client.invalidateQueries({ predicate: (q) => q.queryKey[0] !== 'events' })
  yield {}

  const reader = response.body.pipeThrough(new TextDecoderStream()).pipeThrough(new EventSourceParserStream()).getReader()
  for (;;) {
    const { value, done } = await reader.read()
    if (done) return
    if (value.event === 'warning') yield { warning: (JSON.parse(value.data) as WebWarning).message }
    else if (!value.event) yield invalidate(client, JSON.parse(value.data) as WtmEventsV2)
  }
}

function invalidate(client: QueryClient, line: WtmEventsV2): Live {
  if (line.type === 'repo.added' || line.type === 'repo.removed') void client.invalidateQueries({ queryKey: ['repos'] })
  else if (line.type.startsWith('worktree.') || line.type === 'operation.finished') {
    const id = client.getQueryData<WebRepos>(['repos'])?.find((repo) => repo.common_dir === line.repo?.common_dir)?.id
    void client.invalidateQueries({ queryKey: id ? ['repo', id] : ['repo'] })
  }
  return {}
}

export type Connection = 'connecting' | 'live' | 'reconnecting' | 'disconnected'

// connectionOf is the state of the stream as its query stands.
export function connectionOf(query: { status: string; fetchStatus: string; error: unknown }): Connection {
  if (refused(query.error)) return 'disconnected'
  if (query.status === 'pending') return 'connecting'
  return query.status === 'success' && query.fetchStatus === 'fetching' ? 'live' : 'reconnecting'
}
