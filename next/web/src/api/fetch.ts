import { token } from '@/api/token'
import type { DomainEnvelope, DomainError, WebRefusal } from './types.gen'

export class HttpError extends Error {
  readonly status: number
  readonly refusal: DomainError | undefined

  constructor(status: number, message: string, refusal?: DomainError) {
    super(message)
    this.status = status
    this.refusal = refusal
  }
}

// the token goes in a header, never in a query string that a log would keep
export function request(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)
  headers.set('X-Wtm-Token', token() ?? '')
  return fetch(path, { ...init, headers })
}

// a dry run the core refuses, or whose answers it finds wrong, still answers with its envelope, whose error says why
export async function refusable(path: string, body: unknown): Promise<DomainEnvelope> {
  const response = await request(path, { method: 'POST', body: JSON.stringify(body) })
  const read = (await response.json().catch(() => undefined)) as DomainEnvelope | WebRefusal | undefined
  if (response.ok || ([400, 403].includes(response.status) && read && 'v' in read)) return read as DomainEnvelope
  throw new HttpError(response.status, read?.error?.message ?? response.statusText)
}

export async function json<T>(path: string, body?: unknown): Promise<T> {
  const response = await request(path, body === undefined ? {} : { method: 'POST', body: JSON.stringify(body) })
  const read: unknown = await response.json().catch(() => undefined)
  if (!response.ok) {
    const refusal = (read as WebRefusal | DomainEnvelope | undefined)?.error ?? undefined
    throw new HttpError(response.status, refusal?.message ?? response.statusText, refusal)
  }
  return read as T
}

// the token travels in a header, which a link cannot send: the file is fetched, then handed to the browser to save
export async function saved(path: string, { name, failure }: { name: string; failure: string }) {
  const response = await request(path)
  if (!response.ok) throw new HttpError(response.status, failure)
  const link = document.createElement('a')
  link.href = URL.createObjectURL(await response.blob())
  link.download = name
  link.click()
  // Safari cancels a download whose URL is revoked as it starts
  setTimeout(() => URL.revokeObjectURL(link.href), 60_000)
}
