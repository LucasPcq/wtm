import { token } from '@/api/token'
import type { DomainEnvelope, WebRefusal } from './types.gen'

export class HttpError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// the token goes in a header, never in a query string that a log would keep
export function request(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)
  headers.set('X-Wtm-Token', token() ?? '')
  return fetch(path, { ...init, headers })
}

// a dry run the core refuses still answers with its envelope, whose error says what refused it and why
export async function refusable(path: string, body: unknown): Promise<DomainEnvelope> {
  const response = await request(path, { method: 'POST', body: JSON.stringify(body) })
  const read = (await response.json().catch(() => undefined)) as DomainEnvelope | WebRefusal | undefined
  if (response.ok || (response.status === 403 && read && 'v' in read)) return read as DomainEnvelope
  throw new HttpError(response.status, read?.error?.message ?? response.statusText)
}

export async function json<T>(path: string, body?: unknown): Promise<T> {
  const response = await request(path, body === undefined ? {} : { method: 'POST', body: JSON.stringify(body) })
  const read: unknown = await response.json().catch(() => undefined)
  if (!response.ok)
    throw new HttpError(
      response.status,
      (read as WebRefusal | DomainEnvelope | undefined)?.error?.message ?? response.statusText,
    )
  return read as T
}
