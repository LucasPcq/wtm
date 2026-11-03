import { useQuery } from '@tanstack/react-query'
import { Info } from 'lucide-react'
import { type ReactNode, useRef, useState } from 'react'
import { readQuery } from '@/api/queries/commands'
import { jobLinesQuery } from '@/api/queries/job-logs'
import type { DomainAnswered, ListJob } from '@/api/types.gen'
import { Alert } from '@/features/worktrees/components/alert'
import { CopyCommand } from '@/features/worktrees/components/copy-command'
import { JobHeader } from '@/features/worktrees/running/components/job-header'
import { LogLines } from '@/features/worktrees/running/components/log-lines'
import { LogNote } from '@/features/worktrees/running/components/log-note'
import { LogStatus } from '@/features/worktrees/running/components/log-status'
import { LogToolbar } from '@/features/worktrees/running/components/log-toolbar'
import { useLogView, type Whole } from '@/features/worktrees/running/hooks/use-log-view'
import { latestError, lineAt, matchingLines, patternOf } from '@/features/worktrees/running/utils/log-search'

export type LogLine = { at?: string; text: string; n?: number }

// actions holds what acts on the job (Restart, Stop, Release): the panel places them left of its ⋯ menu
export function JobPanel({
  repo,
  worktree,
  profile,
  job,
  now,
  actions,
  switcher,
}: {
  repo: string
  worktree: string
  profile: string | undefined
  job: ListJob
  now: number
  actions?: ReactNode
  switcher?: ReactNode
}) {
  const [view, dispatch] = useLogView()
  const [timeOpen, setTimeOpen] = useState(false)
  const search = useRef<HTMLInputElement>(null)

  const answers: DomainAnswered[] = [
    { path: 'worktree', values: [worktree] },
    { path: 'job', values: [job.name] },
  ]
  const facts = useQuery(readQuery(repo, 'run-logs', answers)).data?.items[0]?.detail?.jobs?.[0]
  const live = useQuery(jobLinesQuery({ repo, worktree, job: job.name }, job.started_at))
  const whole = useQuery({
    ...readQuery(repo, 'run-logs', [...answers, ...wholeAnswers(view.whole)]),
    enabled: view.whole !== undefined,
  })
  const read = whole.data?.items[0]?.detail?.jobs?.[0]

  const lines: LogLine[] = view.whole ? (read?.lines ?? []) : (live.data?.lines ?? [])
  const pattern = patternOf(view.query)
  const matches = pattern instanceof RegExp ? matchingLines(lines, pattern) : []
  const current = view.focus ?? matches[matches.length - 1 - view.fromLast]
  const crashed = job.state === 'crashed' || live.data?.ended?.state === 'crashed'
  const exitCode = job.exit_code ?? live.data?.ended?.exit_code

  const latest = () => {
    const at = latestError(lines)
    dispatch({ type: 'jump', focus: at, note: at === undefined ? { kind: 'no-error' } : { kind: 'error' } })
  }

  const goTo = (time: string) => {
    const at = lineAt(lines, time)
    if (at === 'older') return dispatch({ type: 'jump', focus: undefined, note: { kind: 'older', time } })
    dispatch({ type: 'jump', focus: at, note: at === undefined ? { kind: 'after', time } : { kind: 'time', time } })
  }

  const status = () => {
    if (view.whole && 'search' in view.whole)
      return `${lines.length} of ${(read?.count ?? 0).toLocaleString()} lines · the whole log`
    if (view.whole) return `${lines.length} lines from the log file`
    if (view.only && pattern instanceof RegExp) return `${matches.length} of ${lines.length} lines`
    if (job.hold === 'detached') return `${lines.length} lines · the launch’s output`
    if (live.data?.ended || crashed)
      return `ended ${lines.at(-1)?.at ? new Date(lines.at(-1)?.at ?? '').toLocaleTimeString() : ''} · ${lines.length} lines`
    if (lines.length >= 1000) return 'The last 1,000 lines · older ones are in the log file'
    return `${lines.length} lines`
  }

  // the panel's keys, while no field is being typed in: / searches, E the latest error, T a time, F follows, W wraps;
  // Alt keys set the search's options, read by their key, which Alt changes the character of on a Mac
  const keys = (panel: HTMLElement | null) => {
    if (!panel) return
    const onKey = (e: KeyboardEvent) => {
      const typing = e.target instanceof HTMLElement && e.target.closest('input, textarea, select, [contenteditable]')
      if (e.metaKey || e.ctrlKey) return
      const alt = () => {
        switch (e.code) {
          case 'KeyC':
            return () => dispatch({ type: 'search', change: { matchCase: !view.query.matchCase } })
          case 'KeyR':
            return () => dispatch({ type: 'search', change: { regex: !view.query.regex } })
          case 'KeyM':
            return () => dispatch({ type: 'only', only: !view.only })
        }
        return undefined
      }
      const plain: Record<string, () => void> = {
        '/': () => search.current?.focus(),
        e: latest,
        t: () => setTimeOpen(true),
        f: () => dispatch({ type: 'follow', follow: !view.follow }),
        w: () => dispatch({ type: 'wrap' }),
      }
      const act = e.altKey ? alt() : !typing && plain[e.key.toLowerCase()]
      if (!act) return
      e.preventDefault()
      act()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }

  return (
    <section
      ref={keys}
      aria-label={`${job.name} in ${worktree}`}
      className="@container/panel flex min-h-0 min-w-0 flex-1 flex-col"
    >
      {switcher}
      <JobHeader
        repo={repo}
        worktree={worktree}
        profile={profile}
        job={job}
        facts={facts}
        now={now}
        actions={actions}
      />
      <div className="flex flex-col gap-3 px-6">
        {crashed && (
          <Alert
            tone="red"
            title={exitCode === undefined ? `${job.name} crashed` : `${job.name} crashed with exit code ${exitCode}`}
            description="Its last lines stay below until it starts again."
          />
        )}
        {facts?.compose_logs && (
          <Alert
            tone="muted"
            icon={Info}
            title="Its services log to Docker, not to wtm"
            description="A detached stack hands its containers to Docker once they start. Follow them with:"
            below={<CopyCommand text={facts.compose_logs} />}
          />
        )}
      </div>
      <LogToolbar
        view={view}
        dispatch={dispatch}
        matches={matches.length}
        invalid={pattern === 'invalid'}
        search={search}
        onLatestError={latest}
        onGoTo={goTo}
        timeOpen={timeOpen}
        onTimeOpen={setTimeOpen}
      />
      <LogNote view={view} dispatch={dispatch} lines={lines} />
      <LogLines
        lines={lines}
        pattern={pattern instanceof RegExp ? pattern : undefined}
        matches={matches}
        current={current}
        view={view}
        dispatch={dispatch}
        total={view.whole ? read?.count : undefined}
        failure={live.error?.message ?? whole.error?.message}
        loading={view.whole ? whole.isPending : live.isPending}
      />
      <LogStatus status={status()} view={view} dispatch={dispatch} />
    </section>
  )
}

function wholeAnswers(whole: Whole | undefined): DomainAnswered[] {
  if (!whole) return []
  if ('since' in whole) return [{ path: 'since', values: [whole.since] }]
  const { text, matchCase, regex } = whole.search
  return [
    { path: 'search', values: [text] },
    { path: 'match_case', values: [String(matchCase)] },
    { path: 'regex', values: [String(regex)] },
  ]
}
