import { CircleDot, CircleX } from 'lucide-react'
import type { RunState, StatusDetail, StatusJob } from '@/api/types.gen'
import { Section } from '@/features/worktrees/detail/section'
import { Glyph, type Tone } from '@/features/worktrees/shared/glyph'
import { ago } from '@/lib/time'
import { withoutScheme } from '@/lib/url'

// whether a job in that state serves its address now
const look: Record<RunState, { tone: Tone; up: boolean }> = {
  starting: { tone: 'muted', up: false },
  running: { tone: 'ok', up: true },
  crashed: { tone: 'failed', up: false },
  exited: { tone: 'muted', up: false },
  stopped: { tone: 'muted', up: false },
}

export function RunSection({ detail: { isolation, run, jobs }, now }: { detail: StatusDetail; now: number }) {
  const meta = [isolation, run?.offset ? `ports +${run.offset}` : undefined].filter(Boolean).join(' · ')

  const where = ({ url, state, exit_code, port }: StatusJob) => {
    if (url && look[state].up)
      return (
        <a
          href={url}
          target="_blank"
          rel="noreferrer"
          className="min-w-0 flex-1 truncate font-mono text-xs underline underline-offset-4"
        >
          {withoutScheme(url)}
        </a>
      )
    const said = exit_code === undefined ? port && `:${port}` : `exit ${exit_code}`
    return <span className="min-w-0 flex-1 truncate font-mono text-xs text-muted-foreground">{said}</span>
  }

  const body = () => {
    if (jobs === null) return <p className="text-sm text-muted-foreground">Which jobs run is unknown.</p>
    if (jobs.length === 0) return <p className="text-sm text-muted-foreground">No job declared.</p>
    return (
      <ul className="flex flex-col gap-2">
        {jobs.map((job) => (
          <li key={job.name} className="flex items-center gap-3 text-sm">
            <Glyph
              icon={look[job.state].tone === 'failed' ? CircleX : CircleDot}
              tone={look[job.state].tone}
              className="w-24 font-medium"
            >
              {job.name}
            </Glyph>
            {where(job)}
            <span className="text-xs text-muted-foreground">
              {look[job.state].up && job.started_at ? ago(job.started_at, now) : job.state}
            </span>
          </li>
        ))}
      </ul>
    )
  }

  return (
    <Section id="run" meta={meta}>
      {body()}
    </Section>
  )
}
