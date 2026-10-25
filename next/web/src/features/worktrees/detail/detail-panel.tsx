import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { CircleCheck, CircleDot, CircleX, Copy, ExternalLink, SlidersHorizontal, TriangleAlert, X } from 'lucide-react'
import type { ReactNode } from 'react'
import { githubQuery, readQuery } from '@/api/queries/commands'
import type { ListDetail, StatusDetail, StatusProblem, StatusReview } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Skeleton } from '@/components/ui/skeleton'
import { ago } from '@/lib/time'
import { useStored } from '../hooks/use-stored'
import { CommandLine } from '../shared/command-line'
import { copy } from '../shared/copy'
import { unavailable } from '../shared/github'
import { Glyph, type Tone } from '../shared/glyph'
import { ActionsMenu } from './actions-menu'
import { sentence } from '../shared/sentence'
import { sectionIds, sectionTitle, shownSections, type SectionId } from './sections'

// DetailPanel is the whole read of one worktree: its status as it changes, what GitHub said of it when the panel
// opened, the problem to fix first with its command, and the sections chosen.
export function DetailPanel(props: { repo: string; worktree: string; row: ListDetail | undefined }) {
  const answers = [{ path: 'worktree', values: [props.worktree] }]
  const live = useQuery(readQuery(props.repo, 'status', answers))
  // GitHub is asked again each time the panel opens, never on an event
  const github = useQuery({ ...githubQuery(props.repo, 'status', answers), refetchOnMount: 'always' })
  const [stored, store] = useStored(`wtm.sections.${props.repo}`)
  const shown = shownSections(stored)

  const detail = live.data?.items[0]?.detail
  const asked = github.data?.items[0]?.detail
  const merged = asked?.review?.fix && asked.problems?.find((problem) => problem.fix?.code === asked.review?.fix?.code)
  const problems = [...(merged ? [merged] : []), ...(detail?.problems ?? [])]
  const pr = props.row?.actions?.find((action) => action.url)
  const actions = props.row?.actions?.filter((action) => !action.url) ?? []

  return (
    <aside aria-label={props.worktree} className="flex w-[440px] shrink-0 flex-col border-l bg-background">
      <header className="flex flex-col gap-1 px-6 pt-5 pb-4">
        <div className="flex items-center gap-2">
          <h2 className="min-w-0 flex-1 truncate text-lg font-semibold">{props.worktree}</h2>
          <Button variant="ghost" size="icon-sm" aria-label="Close the detail" asChild>
            <Link from="/r/$repo/worktrees" search={(search) => ({ ...search, worktree: undefined })}>
              <X />
            </Link>
          </Button>
        </div>
        {detail && <Origin detail={detail} now={live.dataUpdatedAt} />}
      </header>

      <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-6 pb-6">
        {live.isPending && <Skeleton className="h-32 w-full" />}
        {live.isError && <p className="text-sm text-destructive">{live.error.message}</p>}
        {problems[0] && <Problems problems={problems} />}
        {(pr || actions.length > 0) && (
          <div className="flex items-center gap-2">
            {pr?.url && (
              <Button variant="outline" asChild>
                <a href={pr.url} target="_blank" rel="noreferrer">
                  <ExternalLink />
                  Pull request #{pr.params?.number}
                </a>
              </Button>
            )}
            {actions.length > 0 && (
              <div className="ml-auto">
                <ActionsMenu actions={actions} />
              </div>
            )}
          </div>
        )}
        {detail &&
          shown.map((section) => (
            <Section key={section} id={section} meta={meta(section, detail, github.dataUpdatedAt)}>
              {section === 'run' && <Run detail={detail} now={live.dataUpdatedAt} />}
              {section === 'review' && (
                <Review
                  review={asked?.review}
                  pending={github.isPending}
                  warning={unavailable(github.data?.warnings)?.message}
                />
              )}
              {section === 'changes' && <Changes detail={detail} />}
              {section === 'activity' && <Activity detail={detail} now={live.dataUpdatedAt} />}
              {section === 'links' && <Links detail={detail} />}
            </Section>
          ))}
      </div>

      <footer className="border-t px-6 py-3">
        <SectionsChooser shown={shown} onChange={store} />
      </footer>
    </aside>
  )
}

function Origin({ detail, now }: { detail: StatusDetail; now: number }) {
  const from = detail.main ? 'main worktree' : detail.parent ? `from ${detail.parent}` : 'a root'
  return (
    <>
      <p className="text-sm text-muted-foreground">
        {from}
        {detail.created_at && ` · created ${ago(detail.created_at, now)}`}
      </p>
      <button
        type="button"
        className="flex items-start gap-1.5 text-left font-mono text-xs break-all text-muted-foreground hover:text-foreground"
        onClick={() => copy(detail.path)}
      >
        {detail.path}
        <Copy className="mt-0.5 size-3 shrink-0" aria-label="Copy the path" />
      </button>
    </>
  )
}

// Problems are what to fix in the worktree, the first given the most room: its message and the command that fixes it.
function Problems({ problems }: { problems: StatusProblem[] }) {
  const [first, ...more] = problems
  if (!first) return null
  return (
    <section aria-label="To do" className="flex flex-col gap-3 rounded-lg border bg-card p-4">
      <Fix problem={first} />
      {more.length > 0 && (
        <details className="group text-sm">
          <summary className="cursor-pointer text-muted-foreground">
            {more.length} more {more.length === 1 ? 'problem' : 'problems'}
          </summary>
          <div className="mt-3 flex flex-col gap-4">
            {more.map((problem) => (
              <Fix key={problem.code + problem.message} problem={problem} />
            ))}
          </div>
        </details>
      )}
    </section>
  )
}

function Fix({ problem }: { problem: StatusProblem }) {
  return (
    <div className="flex flex-col gap-2">
      <Glyph icon={TriangleAlert} tone="attention" className="font-medium">
        {sentence(problem.message)}
      </Glyph>
      {problem.fix && (
        <>
          <p className="text-sm text-muted-foreground">
            {sentence(problem.fix.message)}. Not in the web interface yet: copy this into a terminal.
          </p>
          <CommandLine followUp={problem.fix} />
        </>
      )}
    </div>
  )
}

function meta(section: SectionId, detail: StatusDetail, askedAt: number): string {
  switch (section) {
    case 'run':
      return [detail.isolation, detail.run?.offset ? `ports +${detail.run.offset}` : undefined]
        .filter(Boolean)
        .join(' · ')
    case 'review':
      return askedAt ? `as of ${new Date(askedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}` : ''
    case 'changes':
      return detail.base ? `${detail.base.ahead} ahead of ${detail.base.name}` : ''
    case 'activity':
      return detail.commits?.length ? `last ${detail.commits.length} commits` : ''
    case 'links':
      return ''
  }
}

function Section(props: { id: SectionId; meta: string; children: ReactNode }) {
  return (
    <section aria-label={sectionTitle[props.id]} className="flex flex-col gap-2.5 border-t pt-4">
      <div className="flex items-baseline gap-2">
        <h3 className="text-sm font-semibold">{sectionTitle[props.id]}</h3>
        <span className="ml-auto text-xs text-muted-foreground">{props.meta}</span>
      </div>
      {props.children}
    </section>
  )
}

const jobTone = (state: string): Tone => (state === 'crashed' ? 'failed' : state === 'running' ? 'ok' : 'muted')

function Run({ detail, now }: { detail: StatusDetail; now: number }) {
  if (detail.jobs === null) return <p className="text-sm text-muted-foreground">Which jobs run is unknown.</p>
  if (detail.jobs.length === 0) return <p className="text-sm text-muted-foreground">No job declared.</p>
  return (
    <ul className="flex flex-col gap-2">
      {detail.jobs.map((job) => (
        <li key={job.name} className="flex items-center gap-3 text-sm">
          <Glyph
            icon={jobTone(job.state) === 'failed' ? CircleX : CircleDot}
            tone={jobTone(job.state)}
            className="w-24 font-medium"
          >
            {job.name}
          </Glyph>
          {job.url && job.state === 'running' ? (
            <a
              href={job.url}
              target="_blank"
              rel="noreferrer"
              className="min-w-0 flex-1 truncate font-mono text-xs underline underline-offset-4"
            >
              {job.url.replace(/^https?:\/\//, '')}
            </a>
          ) : (
            <span className="min-w-0 flex-1 truncate font-mono text-xs text-muted-foreground">
              {job.exit_code !== undefined ? `exit ${job.exit_code}` : job.port ? `:${job.port}` : ''}
            </span>
          )}
          <span className="text-xs text-muted-foreground">
            {job.state === 'running' && job.started_at ? ago(job.started_at, now) : job.state}
          </span>
        </li>
      ))}
    </ul>
  )
}

function Review(props: { review: StatusReview | null | undefined; pending: boolean; warning: string | undefined }) {
  if (props.pending) return <Skeleton className="h-10 w-full" />
  if (props.warning) return <p className="text-sm text-muted-foreground">{props.warning}</p>
  if (!props.review) return <p className="text-sm text-muted-foreground">No pull request.</p>
  const { checks } = props.review
  return (
    <div className="flex flex-col gap-2 text-sm">
      <a
        href={props.review.url}
        target="_blank"
        rel="noreferrer"
        className="font-medium underline-offset-4 hover:underline"
      >
        #{props.review.number} {props.review.title ?? props.review.state}
      </a>
      <div className="flex flex-wrap gap-4">
        {props.review.draft && <span className="text-muted-foreground">draft</span>}
        {props.review.state !== 'open' && <span className="text-muted-foreground">{props.review.state}</span>}
        {checks && (
          <Glyph
            icon={checks.failed ? CircleX : CircleCheck}
            tone={checks.failed ? 'failed' : checks.pending ? 'attention' : 'ok'}
          >
            {checks.failed
              ? `${checks.failed} checks failed`
              : checks.pending
                ? `${checks.pending} checks pending`
                : `${checks.passed} checks passed`}
          </Glyph>
        )}
        {props.review.decision && (
          <span className="text-muted-foreground">{props.review.decision.toLowerCase().replaceAll('_', ' ')}</span>
        )}
      </div>
    </div>
  )
}

function Changes({ detail }: { detail: StatusDetail }) {
  const changes = detail.changes
  const origin = detail.origin
  const standing = origin
    ? origin.ahead || origin.behind
      ? `origin ↑${origin.ahead} ↓${origin.behind}`
      : 'in step with origin'
    : 'not on origin'
  if (!changes) return null
  const files = changes.files ?? []
  return (
    <div className="flex flex-col gap-2 text-sm">
      <p className="text-muted-foreground">
        {files.length === 0
          ? 'Working tree clean'
          : `${changes.staged} staged · ${changes.modified} modified · ${changes.untracked} untracked · +${changes.insertions} −${changes.deletions}`}
        {` · ${standing}`}
      </p>
      {files.length > 0 && (
        <ul className="flex flex-col gap-0.5 font-mono text-xs">
          {files.slice(0, 8).map((file) => (
            <li key={file.path} className="flex gap-2">
              <span className="w-20 shrink-0 text-muted-foreground">{file.status}</span>
              <span className="truncate">{file.path}</span>
            </li>
          ))}
          {files.length > 8 && <li className="text-muted-foreground">and {files.length - 8} more</li>}
        </ul>
      )}
    </div>
  )
}

function Activity({ detail, now }: { detail: StatusDetail; now: number }) {
  if (!detail.commits?.length) return <p className="text-sm text-muted-foreground">No commit yet.</p>
  return (
    <ul className="flex flex-col gap-2">
      {detail.commits.map((commit) => (
        <li key={commit.sha} className="flex flex-col">
          <span className="truncate text-sm">{commit.subject}</span>
          <span className="font-mono text-xs text-muted-foreground">
            {commit.sha.slice(0, 7)} · {commit.author} · {ago(commit.at, now)}
          </span>
        </li>
      ))}
    </ul>
  )
}

function Links({ detail }: { detail: StatusDetail }) {
  const drift = (detail.env.files ?? []).filter(
    (file) => file.missing || file.keys.missing || file.keys.conflicting || file.keys.orphan,
  )
  return (
    <dl className="grid grid-cols-[6rem_1fr] gap-x-3 gap-y-1.5 text-sm">
      <dt className="text-muted-foreground">Parent</dt>
      <dd>{detail.parent || '—'}</dd>
      <dt className="text-muted-foreground">Children</dt>
      <dd>{detail.children?.length ? detail.children.join(', ') : '—'}</dd>
      <dt className="text-muted-foreground">.env</dt>
      <dd>
        {detail.env.declared === 0
          ? 'none declared'
          : drift.length === 0
            ? `${detail.env.declared} ${detail.env.declared === 1 ? 'file' : 'files'}, in step`
            : drift
                .map(
                  (file) =>
                    `${file.target}: ${file.missing ? 'missing' : `${file.keys.missing} missing, ${file.keys.conflicting} differ`}`,
                )
                .join('; ')}
      </dd>
    </dl>
  )
}

function SectionsChooser(props: { shown: SectionId[]; onChange: (shown: SectionId[]) => void }) {
  const hidden = sectionIds.length - props.shown.length
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="sm" className="text-muted-foreground">
          <SlidersHorizontal />
          Sections · {hidden === 0 ? 'all shown' : `${hidden} hidden`}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" side="top" className="w-56 p-2">
        {sectionIds.map((id) => (
          <label
            key={id}
            className="flex cursor-pointer items-center gap-2.5 rounded-md px-2 py-1.5 text-sm hover:bg-accent"
          >
            <Checkbox
              checked={props.shown.includes(id)}
              onCheckedChange={() =>
                props.onChange(
                  props.shown.includes(id)
                    ? props.shown.filter((s) => s !== id)
                    : sectionIds.filter((s) => s === id || props.shown.includes(s)),
                )
              }
            />
            {sectionTitle[id]}
          </label>
        ))}
      </PopoverContent>
    </Popover>
  )
}
