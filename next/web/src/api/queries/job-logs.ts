import { queryOptions, experimental_streamedQuery as streamedQuery } from '@tanstack/react-query'
import { EventSourceParserStream } from 'eventsource-parser/stream'
import { HttpError, request } from '@/api/fetch'
import type { DomainEnvelope, WebJobLine } from '@/api/types.gen'

// seq numbers a line in the stream, so that a line keeps its key while the oldest go
export type JobLine = Extract<WebJobLine, { type: 'line' }> & { seq: number }
export type JobEnd = Extract<WebJobLine, { type: 'ended' }>
export type JobWarning = Extract<WebJobLine, { type: 'warning' }>

export type JobLines = { lines: JobLine[]; count: number; ended: JobEnd | undefined; warnings: JobWarning[] }

// the server answered: the job may print nothing for a while, the stream is no longer being read
type Opened = { type: 'opened' }

export type JobAt = { repo: string; worktree: string; job: string }

// a job printing for days would hold every line in the page: the oldest go once it holds this many
const keptLines = 5000

const path = ({ repo, worktree, job }: JobAt, file: 'logs' | 'log') =>
  `/api/repos/${repo}/jobs/${encodeURIComponent(job)}/${file}?worktree=${encodeURIComponent(worktree)}`

// the job's last lines, then each new one until it ends; a run of the job started again is another stream, keyed by
// when it started
export const jobLinesQuery = (at: JobAt, startedAt: string | undefined) =>
  queryOptions({
    queryKey: ['job-lines', at.repo, at.worktree, at.job, startedAt ?? ''],
    queryFn: streamedQuery<WebJobLine | Opened, JobLines>({
      streamFn: ({ signal }) => lines(at, signal),
      refetchMode: 'reset',
      reducer: (read, line) => {
        if (line.type === 'line')
          return {
            ...read,
            lines: [...read.lines, { ...line, seq: read.count }].slice(-keptLines),
            count: read.count + 1,
          }
        if (line.type === 'ended') return { ...read, ended: line }
        // a warning said again says nothing new
        if (line.type === 'warning' && !read.warnings.some((said) => said.message === line.message))
          return { ...read, warnings: [...read.warnings, line] }
        return read
      },
      initialValue: { lines: [], count: 0, ended: undefined, warnings: [] },
    }),
    staleTime: Infinity,
    retry: false,
    refetchOnWindowFocus: false,
  })

async function* lines(at: JobAt, signal: AbortSignal): AsyncGenerator<WebJobLine | Opened> {
  const response = await request(path(at, 'logs'), { signal })
  if (!response.ok || !response.body) {
    const refused = (await response.json().catch(() => undefined)) as DomainEnvelope | undefined
    throw new HttpError(response.status, refused?.error?.message ?? 'the job’s lines were refused', refused?.error)
  }
  yield { type: 'opened' }

  const reader = response.body
    .pipeThrough(new TextDecoderStream())
    .pipeThrough(new EventSourceParserStream())
    .getReader()
  for (;;) {
    const { value, done } = await reader.read()
    if (done) return
    if (value.event === 'envelope') {
      const ended = JSON.parse(value.data) as DomainEnvelope
      throw new HttpError(500, ended.error?.message ?? 'the job’s lines broke off', ended.error)
    }
    if (!value.event) yield JSON.parse(value.data) as WebJobLine
  }
}

// the token travels in a header, which a link cannot send: the file is fetched, then handed to the browser to save
export async function downloadLog(at: JobAt) {
  const response = await request(path(at, 'log'))
  if (!response.ok) throw new HttpError(response.status, `${at.job}’s log could not be saved`)
  const link = document.createElement('a')
  link.href = URL.createObjectURL(await response.blob())
  link.download = `${at.job}.log`
  link.click()
  // Safari cancels a download whose URL is revoked as it starts
  setTimeout(() => URL.revokeObjectURL(link.href), 60_000)
}
