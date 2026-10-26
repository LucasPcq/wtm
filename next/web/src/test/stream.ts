import { HttpResponse } from 'msw'

export const line = (event: object) => `data: ${JSON.stringify({ v: 2, ts: '2026-10-25T12:00:00Z', ...event })}\n\n`

export const streamed = (body: string, operation: string | null = 'op-1') =>
  new HttpResponse(body, {
    headers: { 'Content-Type': 'text/event-stream', ...(operation && { 'X-Wtm-Operation': operation }) },
  })
