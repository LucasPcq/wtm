import { CircleCheck, CircleX, FileText, Loader, RotateCw, Square, TriangleAlert } from 'lucide-react'
import { toast } from 'sonner'
import { downloadDaemonLog } from '@/api/queries/daemon'
import type { DaemonstatusDetail } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Glyph } from '@/features/worktrees/components/glyph'
import { repoLine, repoName } from '@/features/worktrees/daemon/utils/rows'

// down: the restart stopped the daemon and the new one didn't start
export type CardState =
  | { kind: 'read' }
  | { kind: 'restarting'; back: number }
  | { kind: 'stopping' }
  | { kind: 'failed'; title: string; message: string; down: boolean }

const comeBack = (count: number) => {
  if (count === 0) return 'Nothing to bring back.'
  return count === 1 ? "1 job comes back once it's up." : `${count} jobs come back once it's up.`
}

export function DaemonCard({
  detail,
  unread,
  state,
  onRestart,
  onStopJobs,
}: {
  detail: DaemonstatusDetail | undefined
  unread: string | undefined
  state: CardState
  onRestart: () => void
  onStopJobs: () => void
}) {
  if (!detail && state.kind === 'read' && unread)
    return <Line icon={CircleX} tone="failed" title="Couldn't read wtm's daemon" below={unread} />
  if (!detail && state.kind === 'read') return <Skeleton className="h-40 w-full" />

  const repos = detail?.repos ?? []
  const busy = state.kind === 'restarting' || state.kind === 'stopping'
  const down = state.kind === 'failed' && state.down

  const headline = () => {
    if (state.kind === 'restarting')
      return <Line icon={Loader} tone="busy" title="Restarting" below={comeBack(state.back)} />
    if (state.kind === 'stopping')
      return <Line icon={Loader} tone="busy" title="Stopping every job" below="The proxy stays up." />
    if (down)
      return (
        <Line
          icon={CircleX}
          tone="failed"
          title="The new daemon didn't start"
          below="Named addresses don't answer; see its log."
        />
      )
    if (state.kind === 'failed') return <Line icon={CircleX} tone="failed" title={state.title} below={state.message} />
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

      {!down && (
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
            {detail?.proxy && !down ? `on :${detail.proxy.port}` : 'not served'}
          </span>
        </p>
      </section>

      <Separator />
      <div className="flex flex-wrap gap-2">
        {down && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => downloadDaemonLog().catch((error: Error) => toast.error(error.message))}
          >
            <FileText />
            Open its log
          </Button>
        )}
        <Button variant="outline" size="sm" disabled={busy} onClick={onRestart}>
          <RotateCw />
          Restart
        </Button>
        <Button variant="outline" size="sm" disabled={busy || down || repos.length === 0} onClick={onStopJobs}>
          <Square />
          Stop every job
        </Button>
      </div>
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
