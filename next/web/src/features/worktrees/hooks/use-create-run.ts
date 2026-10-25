import { skipToken, useMutation, useMutationState, useQuery, useQueryClient } from '@tanstack/react-query'
import { repoKey } from '@/api/queries/commands'
import { cancel, runStreamed } from '@/api/queries/operations'
import type { CreatePlan, DomainAnswered } from '@/api/types.gen'
import { advance, planned, type Progress } from '../create/progress'

// The progress of the create this page runs lives in the Query cache, out of the repository's keys: the dialog
// shows it, closed and opened again, and the list marks the rows it works on.
const progressKey = (repo: string) => ['progress', repo, 'create'] as const
const runKey = (repo: string) => ['create', repo] as const

export type Start = { answers: DomainAnswered[]; fingerprint: string; plan: CreatePlan }

// useCreateRun is the create of repo this page runs: its progress, whether it still runs, and how to start it,
// cancel it (then force it) and put it away once it ended.
export function useCreateRun(repo: string) {
  const client = useQueryClient()
  const { progress, running } = useRun(repo)
  const update = (change: (progress: Progress) => Progress) =>
    client.setQueryData<Progress>(progressKey(repo), (current) => current && change(current))

  const start = useMutation({
    mutationKey: runKey(repo),
    mutationFn: (params: Start) => {
      client.setQueryData(progressKey(repo), planned(params.plan))
      return runStreamed({
        repo,
        command: 'create',
        call: { answers: params.answers, expect_fingerprint: params.fingerprint },
        started: (operation) => update((current) => ({ ...current, operation })),
        line: (line) => update((current) => advance(current, line)),
      })
    },
    onSuccess: (envelope) => update((current) => ({ ...current, envelope })),
    onError: (error) => update((current) => ({ ...current, refused: error.message })),
    onSettled: () => client.invalidateQueries({ queryKey: repoKey(repo) }),
  })

  return {
    progress,
    running,
    start: (params: Start) => start.mutate(params),
    cancel: (force: boolean) => {
      if (!progress?.operation) return
      update((current) => ({ ...current, cancelling: force ? 'forced' : 'asked' }))
      void cancel(repo, progress.operation, force)
    },
    dismiss: () => client.removeQueries({ queryKey: progressKey(repo) }),
  }
}

// useRun is the progress of the create this page runs and whether it still runs, read without running one.
function useRun(repo: string) {
  const progress = useQuery<Progress>({ queryKey: progressKey(repo), queryFn: skipToken }).data
  const running = useMutationState({ filters: { mutationKey: runKey(repo), status: 'pending' } }).length > 0
  return { progress, running }
}

// useUnderWay is what the create this page runs does to each worktree, by branch, while it runs.
export function useUnderWay(repo: string): Record<string, string> {
  const { progress, running } = useRun(repo)
  if (!running || !progress) return {}
  return Object.fromEntries(
    progress.units
      .filter(([, unit]) => !unit.status)
      .map(([subject, unit]) => {
        const done = unit.phases.filter((run) => run.state === 'done').length
        return [subject, `Creating · ${done}/${unit.phases.length}`]
      }),
  )
}
