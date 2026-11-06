import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { dryRunQuery, questionsQuery, readQuery } from '@/api/queries/commands'
import type { DomainAnswered, DomainFollowUp, ListJob } from '@/api/types.gen'
import { useDirectRun } from '@/features/worktrees/hooks/use-direct-run'
import { useRun } from '@/features/worktrees/hooks/use-run'
import { openRunStart, setRunStartDialog, showRunStart } from '@/features/worktrees/hooks/use-run-start-dialog'
import { setRunStopDialog, showRunStop } from '@/features/worktrees/hooks/use-run-stop-dialog'
import { useJobDoing } from '@/features/worktrees/running/hooks/use-job-doing'
import { doingToast, type JobAct, type JobDoing } from '@/features/worktrees/running/utils/job-words'
import { type JobAt, startedTold, stoppedTold } from '@/features/worktrees/utils/job-told'
import { jobsTodo } from '@/features/worktrees/utils/progress'

// holders count the worktrees holding a shared service, which its restart stops for a moment
export type JobActs = {
  acts: { act: JobAct; why: string | undefined; keptBy: string[]; run: () => void }[]
  doing: JobDoing | undefined
  shared: boolean
  holders: number
}

// a restart or an undo puts back a job that ran beside the others' a moment ago: the others' jobs are kept
const parallel: DomainAnswered = { path: 'parallel', values: ['true'] }
const restart: DomainAnswered = { path: 'restart', values: ['true'] }

// what acts on one job: a job up restarts and stops, or releases the worktree's hold on a shared service, as run
// stop's plan says; a job not up starts again. A shared service's restart cuts the others' work: it asks first
export function useJobActions({ repo, worktree, job }: { repo: string; worktree: string; job: ListJob }): JobActs {
  const at: JobAt = { worktree, job: job.name }
  const answers: DomainAnswered[] = [
    { path: 'worktree', values: [worktree] },
    { path: 'job', values: [job.name] },
  ]
  const up = job.state === 'running' || job.state === 'starting'
  const stopPlan = useQuery({ ...dryRunQuery(repo, 'run-stop', answers), enabled: up })
  const fate = stopPlan.data?.plan?.worktrees?.[0]?.jobs?.[0]
  // a worktree restarts a shared service only while it holds it: the core refuses it elsewhere
  const form = useQuery({ ...questionsQuery(repo, 'run-start', [...answers, restart]), enabled: up })
  const unheld = form.data?.problems.some((problem) => problem.code === 'run-start.restart_shared')
  // main's own panel of a shared service it runs for the others alone: they hold it, and one of them restarts it
  const claimers = (useQuery(readQuery(repo, 'list')).data?.items ?? []).flatMap(({ subject, detail }) =>
    detail?.jobs?.some((one) => one.name === job.name && one.hold === 'claim') ? [subject] : [],
  )
  const shared = fate?.shared_in !== undefined || (unheld === true && claimers.length > 0)
  const holders = fate ? (fate.kept_by?.length ?? 0) + 1 : claimers.length
  const doing = useJobDoing(repo, worktree, job.name, fate?.fate)
  const startRun = useDirectRun(repo, 'run-start')
  const stopRun = useDirectRun(repo, 'run-stop')
  // one start and one stop at a time, as the run's store holds one of each: another job's waits its end
  const starting = useRun(repo, 'run-start').running
  const stopping = useRun(repo, 'run-stop').running

  const refusedToast = (title: string, message: string | undefined) =>
    toast.error(title, { description: message ?? 'Nothing to do: the jobs changed meanwhile.' })

  const start = (asked: DomainAnswered[], again: boolean) => {
    const { title, where } = doingToast(again ? 'restarting' : 'starting', at)
    const all = [...asked, parallel]
    void startRun({
      subjects: [worktree],
      answers: all,
      applying: all,
      // called once the run is this one's, never for a click that found another under way: its outcome is titled so
      unitsOf: (plan) => {
        setRunStartDialog(repo, { ran: { ...at, restart: again, plan } })
        return (plan.worktrees ?? []).flatMap((target) => {
          const names = (target.jobs ?? []).filter((one) => one.action !== 'up').map((one) => one.name)
          return names.length > 0 ? [jobsTodo(target.worktree, names)] : []
        })
      },
      doing: title,
      doingWhere: where,
      told: (envelope, refused) => startedTold({ at, restart: again }, envelope, refused),
      shown: () => false,
      onDecide: (dry) => refusedToast(title, dry?.error?.message ?? dry?.warnings[0]?.message),
      onDetails: () => showRunStart(repo),
    })
  }

  const undo = (followUp: DomainFollowUp | undefined) =>
    followUp && { label: 'Undo', onClick: () => start(followUp.answers ?? [], false) }

  const stop = () => {
    const { title, where } = doingToast(fate?.fate === 'release' ? 'releasing' : 'stopping', at)
    void stopRun({
      subjects: [worktree],
      answers,
      applying: answers,
      unitsOf: (plan) => {
        setRunStopDialog(repo, { ran: at })
        return (plan.worktrees ?? []).flatMap((target) =>
          (target.jobs ?? []).length > 0 ? [jobsTodo(target.worktree, [job.name])] : [],
        )
      },
      doing: title,
      doingWhere: where,
      told: (envelope, refused) =>
        stoppedTold(at, envelope, refused, undo(envelope?.follow_ups.find((one) => one.code === 'run-stop.start'))),
      shown: () => false,
      onDecide: (dry) => refusedToast(title, dry?.error?.message ?? dry?.warnings[0]?.message),
      onDetails: () => showRunStop(repo),
    })
  }

  // a shared service main runs for the others alone is theirs to let go of: run stop's plan reaches none of it
  const unreached = stopPlan.isSuccess && !fate ? `Runs ${job.name} for the other worktrees only` : undefined
  const busy = doing ? 'Under way' : undefined
  const startBusy = busy ?? (starting ? 'Another job is starting: one at a time' : undefined)
  const stopBusy = busy ?? (stopping ? 'Another job is stopping: one at a time' : undefined)
  const [claimer] = claimers

  // whether it is a shared service, whose restart asks first, is the stop's plan's to say
  const restartWhy = () => {
    if (unheld && !claimer) return `${worktree} doesn’t hold ${job.name}: a worktree holding it restarts it`
    if (!stopPlan.isSuccess) return 'Reading the job…'
    return undefined
  }

  const restartIt = () => {
    if (unheld && claimer) return openRunStart(repo, { worktree: claimer, job: job.name, restart: true })
    if (shared) return openRunStart(repo, { worktree, job: job.name, restart: true })
    start([...answers, restart], true)
  }

  if (!up)
    return {
      acts: [{ act: 'start', why: startBusy, keptBy: [], run: () => start(answers, false) }],
      doing,
      shared,
      holders,
    }
  return {
    acts: [
      { act: 'restart', why: startBusy ?? restartWhy(), keptBy: [], run: restartIt },
      {
        act: fate?.fate === 'release' ? 'release' : 'stop',
        why: stopBusy ?? unreached,
        keptBy: fate?.kept_by ?? [],
        run: stop,
      },
    ],
    doing,
    shared,
    holders,
  }
}
