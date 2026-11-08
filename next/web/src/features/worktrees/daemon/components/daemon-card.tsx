import { CircleCheck, CircleX, FileText, Loader, RotateCw, Square, TriangleAlert } from 'lucide-react'
import { toast } from 'sonner'
import { downloadDaemonLog } from '@/api/queries/daemon'
import type { DaemonstatusDetail } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Glyph } from '@/features/worktrees/components/glyph'
import { repoLine, repoName } from '@/features/worktrees/daemon/utils/rows'

// the panel says the daemon as it stands, or the restart under way, or the one that failed
export type CardState = { kind: 'read' } | { kind: 'restarting' } | { kind: 'failed'; message: string }

export function DaemonCard({
  detail,
  state,
  onRestart,
  onStopJobs,
}: {
  detail: DaemonstatusDetail | undefined
  state: CardState
  onRestart: () => void
  onStopJobs: () => void
}) {
  if (!detail && state.kind === 'read') return <Skeleton className="h-40 w-full" />

  const repos = detail?.repos ?? []
  const busy = state.kind === 'restarting'

  const headline = () => {
    if (state.kind === 'restarting')
      return <Line icon={Loader} tone="busy" title="Restarting" below="Its jobs come back once it's up." />
    if (state.kind === 'failed')
      return <Line icon={CircleX} tone="failed" title="The new daemon didn't start" below={state.message} />
    if (!detail?.running)
      return <Line icon={Square} tone="muted" title="Not running" below="The next job started brings it up." />
    if (detail.skew)
      return (
        <Line
          icon={TriangleAlert}
          tone="attention"
          title={`Runs ${detail.daemon ?? 'another build'}; this version is ${detail.build}`}
          below="Starting a job is refused until it restarts."
        />
      )
    return <Line icon={CircleCheck} tone="ok" title={`Up · ${detail.build}, this version`} />
  }

  return (
    <div className="flex flex-col gap-4 text-sm">
      <p className="flex items-baseline justify-between gap-4">
        <span className="font-medium">wtm's daemon</span>
        <span className="text-xs text-muted-foreground">every repository on this machine</span>
      </p>
      {headline()}

      {state.kind !== 'failed' && (
        <section className="flex flex-col gap-2" aria-label="Jobs it runs">
          <h3 className="text-xs text-muted-foreground">Jobs it runs</h3>
          {repos.length === 0 && <p className="text-muted-foreground">None</p>}
          {repos.map((repo) => (
            <p key={repo.root} className="flex items-baseline justify-between gap-4">
              <span>{repoName(repo.root)}</span>
              <span className="truncate text-muted-foreground">{repoLine(repo)}</span>
            </p>
          ))}
          {(detail?.left ?? []).length > 0 && (
            <Glyph icon={TriangleAlert} tone="attention">
              A daemon that died left jobs; Restart brings them back.
            </Glyph>
          )}
        </section>
      )}

      <section className="flex flex-col gap-2" aria-label="Proxy">
        <h3 className="text-xs text-muted-foreground">Proxy</h3>
        <p className="flex items-baseline justify-between gap-4">
          <span>Named addresses</span>
          <span className="text-muted-foreground">
            {detail?.proxy && state.kind !== 'failed' ? `on :${detail.proxy.port}` : 'not served'}
          </span>
        </p>
      </section>

      <Separator />
      {state.kind === 'failed' ? (
        <div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => downloadDaemonLog().catch((error: Error) => toast.error(error.message))}
          >
            <FileText />
            Open its log
          </Button>
        </div>
      ) : (
        <div className="flex gap-2">
          <Button variant="outline" size="sm" disabled={busy} onClick={onRestart}>
            <RotateCw />
            Restart
          </Button>
          <Button variant="outline" size="sm" disabled={busy || repos.length === 0} onClick={onStopJobs}>
            <Square />
            Stop every job
          </Button>
        </div>
      )}
    </div>
  )
}

function Line({
  icon,
  tone,
  title,
  below,
}: {
  icon: typeof CircleCheck
  tone: 'ok' | 'attention' | 'failed' | 'muted' | 'busy'
  title: string
  below?: string
}) {
  return (
    <div className="flex flex-col gap-0.5">
      <Glyph icon={icon} tone={tone}>
        {title}
      </Glyph>
      {below && <p className="pl-5 text-xs text-muted-foreground">{below}</p>}
    </div>
  )
}
