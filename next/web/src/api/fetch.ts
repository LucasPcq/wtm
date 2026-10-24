import { token } from './token'
import type { DomainEnvelope, WebRefusal } from './types.gen'

export class HttpError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// request is fetch with the token of the server's start in X-Wtm-Token, never in a query string.
export function request(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)
  headers.set('X-Wtm-Token', token() ?? '')
  return fetch(path, { ...init, headers })
}

// json returns the body of an accepted response, and throws the message of the server's error for any other.
export async function json<T>(path: string, body?: unknown): Promise<T> {
  const response = await request(path, body === undefined ? {} : { method: 'POST', body: JSON.stringify(body) })
  const read: unknown = await response.json().catch(() => undefined)
  if (!response.ok) throw new HttpError(response.status, (read as WebRefusal | DomainEnvelope | undefined)?.error?.message ?? response.statusText)
  return read as T
}
