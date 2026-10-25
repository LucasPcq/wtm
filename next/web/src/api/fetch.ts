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
