import {
  type Query,
  type QueryClient,
  queryOptions,
  experimental_streamedQuery as streamedQuery,
} from '@tanstack/react-query'
import { EventSourceParserStream } from 'eventsource-parser/stream'
import { HttpError, request } from '@/api/fetch'
import type { DomainAnswered, WebRepos, WebWarning, WtmEventsV2 } from '@/api/types.gen'

export type UnderWay = { repo: string | undefined; command: string; subjects: string[]; since: number }

// a process that died never says its operation finished
export const operationCap = 10 * 60_000

export const current = (operations: Record<string, UnderWay>, now: number) =>
  Object.values(operations).filter((operation) => now - operation.since < operationCap)

// ready: the stream reached the core's ready, every change from now on reaches the page
export type Live = { warning?: string; warningCode?: string; ready?: boolean; operations: Record<string, UnderWay> }

export const reconnectEvery = 2000

// a server that answers none of this many reconnects in a row is down: the page stops asking until Try again
export const giveUpAfter = 5

// a stream that could not open, the attempts in a row that failed with it
export class Unreachable extends Error {
  readonly attempts: number

  constructor(attempts: number) {
    super("wtm's server doesn't answer")
    this.attempts = attempts
  }
}

const down = (error: unknown) => error instanceof Unreachable && error.attempts >= giveUpAfter

// retried every reconnectEvery until the server refuses the token: it restarted, and the token change that follows
// reloads the page (main.tsx); or until it answers none of giveUpAfter reconnects
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
  refetchInterval: (query) => (refused(query.state.error) || down(query.state.error) ? false : reconnectEvery),
  refetchIntervalInBackground: true,
})

const refused = (error: unknown) => error instanceof HttpError && error.status === 401

// a query that never had data starts each fetch with its error cleared: the count lives beside it
const failures = new WeakMap<QueryClient, number>()

async function opened(client: QueryClient, signal: AbortSignal): Promise<NonNullable<Response['body']>> {
  const failed = () => {
    const attempts = (failures.get(client) ?? 0) + 1
    failures.set(client, attempts)
    return new Unreachable(attempts)
  }
  const response = await request('/api/events', { signal }).catch((error: unknown) => {
    if (signal.aborted) throw error
    throw failed()
  })
  if (response.status === 401) throw new HttpError(response.status, 'the event stream was refused')
  if (!response.ok || !response.body) throw failed()
  failures.delete(client)
  return response.body
}

async function* lines(client: QueryClient, signal: AbortSignal): AsyncGenerator<Live> {
  const body = await opened(client, signal)
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
  let ready = false
  yield { operations, ready }

  const reader = body.pipeThrough(new TextDecoderStream()).pipeThrough(new EventSourceParserStream()).getReader()
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
      yield { operations, ready }
      continue
    }

    next = reader.read()
    const { value, done } = arrived.read
    if (done) {
      // the stream ended: what settles still releases, within settleCap
      for (const line of await Promise.all(settling.values()))
        operations = track(operations, line, repoOf(client, line))
      if (settling.size > 0) yield { operations, ready }
      return
    }
    if (value.event === 'warning') {
      const warning = JSON.parse(value.data) as WebWarning
      if (warning.code === 'daemon.start_failed') void daemonRead(client)
      // the bus lost: not ready until the core subscribes again and says so; any other warning leaves it as it was
      if (warning.code === 'events.bus_lost' || warning.code === 'daemon.start_failed') ready = false
      yield { warning: warning.message, ...(warning.code && { warningCode: warning.code }), operations, ready }
    }
    if (!value.event) {
      const line = JSON.parse(value.data) as WtmEventsV2
      const at = Date.now()
      void invalidate(client, line)
      // a worktree stays under way until the read that no longer shows it lands, it would flash back in between, but
      // never longer than settleCap, nor holding the lines after it
      if (line.type === 'operation.finished' && line.op_id) {
        const repo = repoOf(client, line)
        settling.set(
          line.op_id,
          settled(client, repo, at).then(() => line),
        )
        continue
      }
      if (line.type === 'ready') ready = true
      operations = track(operations, line, repoOf(client, line))
      yield { operations, ready }
    }
  }
}

export const settleCap = 10_000

// the list or tree read of repo that lands after since, and the status read of an open detail with it, else settleCap: a
// review's fetch of origin is never waited on
function settled(client: QueryClient, repo: string | undefined, since: number): Promise<void> {
  const of = (query: Query, commands: string[]) =>
    query.queryKey[0] === 'repo' && query.queryKey[1] === repo && commands.includes(String(query.queryKey[2]))
  const status = (query: Query) => of(query, ['status'])
  let read = false
  let detail = !client
    .getQueryCache()
    .getAll()
    .some((query) => status(query) && query.getObserversCount() > 0)
  return new Promise((resolve) => {
    const done = () => {
      clearTimeout(timer)
      unsubscribe()
      resolve()
    }
    const timer = setTimeout(done, settleCap)
    const unsubscribe = client.getQueryCache().subscribe((event) => {
      if (event.type !== 'updated' || event.query.state.dataUpdatedAt < since) return
      if (of(event.query, ['list', 'tree'])) read = true
      if (status(event.query)) detail = true
      if (read && detail) done()
    })
  })
}

const repoOf = (client: QueryClient, line: WtmEventsV2) =>
  client.getQueryData<WebRepos>(['repos'])?.find((repo) => repo.common_dir === line.repo?.common_dir)?.id

// wtm's daemon is the machine's: its read under every repository
const daemonRead = (client: QueryClient) =>
  client.invalidateQueries({
    queryKey: ['repo'],
    predicate: (query) => query.queryKey[2] === 'daemon-status' && query.queryKey[3] !== 'questions',
  })

function invalidate(client: QueryClient, line: WtmEventsV2) {
  // ready after a break: wtm's daemon answers again, its read may still say it down
  if (line.type === 'ready') return daemonRead(client)
  if (line.type === 'repo.added' || line.type === 'repo.removed')
    return client.invalidateQueries({ queryKey: ['repos'] })
  // a job's event changes the reads that list jobs or say a job's facts, never a dialog's questions nor its plan, nor a
  // read of a whole log: a start of N jobs would ask them N times; wtm's daemon is the machine's, read under every repository
  if (line.type.startsWith('job.')) {
    const id = repoOf(client, line)
    return Promise.all([
      ...(['list', 'status', 'run-logs', 'run-ps'] as const).map((read) =>
        client.invalidateQueries({
          queryKey: id ? ['repo', id, read] : ['repo'],
          predicate: (query) => !readsWhole(query),
        }),
      ),
      daemonRead(client),
    ])
  }
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

// disconnected: the server restarted under a new session; daemon-down and server-down wait for Try again
export type Connection = 'connecting' | 'live' | 'reconnecting' | 'disconnected' | 'daemon-down' | 'server-down'

export function connectionOf(query: {
  status: string
  fetchStatus: string
  error: unknown
  data?: { ready?: boolean | undefined; warningCode?: string | undefined } | undefined
}): Connection {
  if (refused(query.error)) return 'disconnected'
  if (down(query.error) && query.fetchStatus !== 'fetching') return 'server-down'
  if (query.status === 'pending') return 'connecting'
  if (query.status !== 'success' || query.fetchStatus !== 'fetching') return 'reconnecting'
  if (query.data?.warningCode === 'daemon.start_failed') return 'daemon-down'
  return query.data?.ready ? 'live' : 'connecting'
}

// a read of run logs that searches a job's whole log or reads it from a time
const readsWhole = (query: Query) =>
  query.queryKey[2] === 'run-logs' &&
  Array.isArray(query.queryKey[3]) &&
  (query.queryKey[3] as DomainAnswered[]).some((answer) => answer.path === 'search' || answer.path === 'since')
