import { EventSourceParserStream } from 'eventsource-parser/stream'
import { HttpError, request } from '../fetch'
import { applied, type Applied, type MutationCommand } from './commands'
import type { DomainEnvelope, DomainEvent, WebCall, WebRefusal } from '../types.gen'

// An OperationLine is one event of an operation as its progress stream sends it, stamped with when it happened.
export type OperationLine = DomainEvent & { ts: string }

// runStreamed runs a command of the catalogue and hands each event of its operation to line as it happens, the
// operation's id to started as soon as the server makes it (to cancel it while it plans); it ends with the
// operation's envelope, and throws the server's refusal of the request, or an answer naming no operation, which no
// cancel could reach.
export async function runStreamed(params: {
  repo: string
  command: MutationCommand
  call: WebCall
  started: (operation: string) => void
  line: (line: OperationLine) => void
}): Promise<Applied<MutationCommand>> {
  const response = await request(`/api/repos/${params.repo}/commands/${params.command}`, {
    method: 'POST',
    headers: { Accept: 'text/event-stream' },
    body: JSON.stringify(params.call),
  })
  if (!response.ok || !response.body) {
    const refusal = (await response.json().catch(() => undefined)) as WebRefusal | DomainEnvelope | undefined
    throw new HttpError(response.status, refusal?.error?.message ?? response.statusText)
  }
  const operation = response.headers.get('X-Wtm-Operation')
  if (!operation) throw new Error('the server started the operation without naming it: it could not be cancelled')
  params.started(operation)

  const reader = response.body
    .pipeThrough(new TextDecoderStream())
    .pipeThrough(new EventSourceParserStream())
    .getReader()
  for (;;) {
    const { value, done } = await reader.read()
    if (done) throw new Error('the operation stopped answering before it ended')
    if (value.event === 'envelope') return applied(JSON.parse(value.data) as DomainEnvelope)
    if (!value.event) params.line(JSON.parse(value.data) as OperationLine)
  }
}

// cancel cancels the operation, as a first Ctrl+C; forced, as the second.
export const cancel = (repo: string, operation: string, force: boolean) =>
  request(`/api/repos/${repo}/operations/${operation}${force ? '?force=true' : ''}`, { method: 'DELETE' })
