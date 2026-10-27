import { EventSourceParserStream } from 'eventsource-parser/stream'
import { HttpError, request } from '@/api/fetch'
import { type Applied, type MutationCommand, narrowed } from '@/api/queries/commands'
import type { DomainEnvelope, DomainEvent, WebCall, WebRefusal } from '@/api/types.gen'

export type OperationLine = DomainEvent & { ts: string }

export async function runStreamed<C extends MutationCommand>({
  repo,
  command,
  call,
  started,
  line,
}: {
  repo: string
  command: C
  call: WebCall
  started: (operation: string) => void
  line: (line: OperationLine) => void
}): Promise<Applied<C>> {
  const connection = new AbortController()
  const response = await request(`/api/repos/${repo}/commands/${command}`, {
    method: 'POST',
    signal: connection.signal,
    headers: { Accept: 'text/event-stream' },
    body: JSON.stringify(call),
  })
  if (!response.ok || !response.body) {
    const refusal = (await response.json().catch(() => undefined)) as WebRefusal | DomainEnvelope | undefined
    // the core's refusal before its apply, which names its command, is the run's envelope: its code says what to do next
    if (refusal?.command) return narrowed(refusal)
    throw new HttpError(response.status, refusal?.error?.message ?? response.statusText)
  }

  const operation = response.headers.get('X-Wtm-Operation')
  // refused, the stream is closed too: left open, the operation would run on with nothing to read it
  if (!operation) {
    connection.abort()
    throw new Error('the server started the operation without naming it: it could not be cancelled')
  }
  started(operation)

  const reader = response.body
    .pipeThrough(new TextDecoderStream())
    .pipeThrough(new EventSourceParserStream())
    .getReader()
  for (;;) {
    const { value, done } = await reader.read()
    if (done) throw new Error('the operation stopped answering before it ended')
    if (value.event === 'envelope') return narrowed(JSON.parse(value.data) as DomainEnvelope)
    if (!value.event) line(JSON.parse(value.data) as OperationLine)
  }
}

export const cancel = (repo: string, operation: string, force: boolean) =>
  request(`/api/repos/${repo}/operations/${operation}${force ? '?force=true' : ''}`, { method: 'DELETE' })
