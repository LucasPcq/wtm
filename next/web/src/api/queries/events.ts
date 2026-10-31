import {
  type Query,
  type QueryClient,
  queryOptions,
  experimental_streamedQuery as streamedQuery,
} from '@tanstack/react-query'
import { EventSourceParserStream } from 'eventsource-parser/stream'
import { HttpError, request } from '@/api/fetch'
import type { WebRepos, WebWarning, WtmEventsV2 } from '@/api/types.gen'

export type UnderWay = { repo: string | undefined; command: string; subjects: string[]; since: number }

// a process that died never says its operation finished
export const operationCap = 10 * 60_000

export const current = (operations: Record<string, UnderWay>, now: number) =>
  Object.values(operations).filter((operation) => now - operation.since < operationCap)

export type Live = { warning?: string; operations: Record<string, UnderWay> }

export const reconnectEvery = 2000

// retried every reconnectEvery until the server refuses the token: it restarted, and the token change that follows
// reloads the page (main.tsx)
export const eventsQuery = queryOptions({
  queryKey: ['events'],
  queryFn: streamedQuery<Live, Live>({
    streamFn: ({ client, signal }) => lines(client, signal),
    refetchMode: 'append',
    reducer: (_, live) => live,
    initialValue: { operations: {} },
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
  // back after a break: the lines in between are lost, so everything is read again, but GitHub, asked on demand only, a
  // review, which fetches origin once per opening, and a plan, asked again only on the reads it was chosen on
  if (client.getQueryData(eventsQuery.queryKey))
    client.invalidateQueries({
      predicate: (q) => !['events', 'github', 'review', 'plan'].includes(String(q.queryKey[0])),
    })
  // shortcut: the bus names a worktree once its unit starts, never what the operation locked from its start, and an
  // operation under way across a reconnect is known again only at its next unit; a field naming the locked subjects
  // on operation.started would lift both
  let operations: Record<string, UnderWay> = {}
  yield { operations }

  const reader = response.body
    .pipeThrough(new TextDecoderStream())
    .pipeThrough(new EventSourceParserStream())
    .getReader()
  // an operation finished settles while the reader goes on: each resolves to the line that releases it
  const settling = new Map<string, Promise<WtmEventsV2>>()
  let next = reader.read()
  for (;;) {
    const arrived = await Promise.race([
      next.then((read) => ({ read })),
      ...[...settling.values()].map((released) => released.then((line) => ({ line }))),
    ])
    if ('line' in arrived) {
      settling.delete(arrived.line.op_id ?? '')
      operations = track(operations, arrived.line, repoOf(client, arrived.line))
      yield { operations }
      continue
    }

    next = reader.read()
    const { value, done } = arrived.read
    if (done) {
      // the stream ended: what settles still releases, within settleCap
      for (const line of await Promise.all(settling.values()))
        operations = track(operations, line, repoOf(client, line))
      if (settling.size > 0) yield { operations }
      return
    }
    if (value.event === 'warning') yield { warning: (JSON.parse(value.data) as WebWarning).message, operations }
    else if (!value.event) {
      const line = JSON.parse(value.data) as WtmEventsV2
      void invalidate(client, line)
      // a worktree stays under way until the read that no longer shows it lands, it would flash back in between, but
      // never longer than settleCap, nor holding the lines after it
      if (line.type === 'operation.finished' && line.op_id) {
        const repo = repoOf(client, line)
        settling.set(
          line.op_id,
          settled(client, repo, Date.now()).then(() => line),
        )
        continue
      }
      operations = track(operations, line, repoOf(client, line))
      yield { operations }
    }
  }
}

export const settleCap = 2000

// the list or tree read of repo that lands after since, else settleCap: a review's fetch of origin is never waited on
function settled(client: QueryClient, repo: string | undefined, since: number): Promise<void> {
  const landed = (query: Query) =>
    query.queryKey[0] === 'repo' &&
    query.queryKey[1] === repo &&
    ['list', 'tree'].includes(String(query.queryKey[2])) &&
    query.state.dataUpdatedAt > since
  return new Promise((resolve) => {
    const done = () => {
      clearTimeout(timer)
      unsubscribe()
      resolve()
    }
    const timer = setTimeout(done, settleCap)
    const unsubscribe = client.getQueryCache().subscribe((event) => {
      if (event.type === 'updated' && landed(event.query)) done()
    })
  })
}

const repoOf = (client: QueryClient, line: WtmEventsV2) =>
  client.getQueryData<WebRepos>(['repos'])?.find((repo) => repo.common_dir === line.repo?.common_dir)?.id

function invalidate(client: QueryClient, line: WtmEventsV2) {
  if (line.type === 'repo.added' || line.type === 'repo.removed')
    return client.invalidateQueries({ queryKey: ['repos'] })
  if (line.type.startsWith('worktree.') || line.type === 'operation.finished') {
    const id = repoOf(client, line)
    return client.invalidateQueries({ queryKey: id ? ['repo', id] : ['repo'] })
  }
  return undefined
}

// the lock is held from the operation's start to its finish: a worktree stays under way past its own unit
export function track(
  operations: Record<string, UnderWay>,
  line: WtmEventsV2,
  repo: string | undefined,
): Record<string, UnderWay> {
  const op = line.op_id
  if (!op) return operations
  const known = operations[op] ?? { repo, command: '', subjects: [], since: Date.parse(line.ts) }

  if (line.type === 'operation.started' && typeof line.command === 'string')
    return { ...operations, [op]: { ...known, command: line.command } }
  if (line.type === 'unit.started' && typeof line.subject === 'string')
    return { ...operations, [op]: { ...known, subjects: [...known.subjects, line.subject] } }
  if (line.type === 'operation.finished')
    return Object.fromEntries(Object.entries(operations).filter(([id]) => id !== op))
  return operations
}

export type Connection = 'connecting' | 'live' | 'reconnecting' | 'disconnected'

export function connectionOf(query: { status: string; fetchStatus: string; error: unknown }): Connection {
  if (refused(query.error)) return 'disconnected'
  if (query.status === 'pending') return 'connecting'
  return query.status === 'success' && query.fetchStatus === 'fetching' ? 'live' : 'reconnecting'
}
