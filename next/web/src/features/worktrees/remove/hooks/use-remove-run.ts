import { useQueryClient } from '@tanstack/react-query'
import { create } from 'zustand'
import { repoKey } from '@/api/queries/commands'
import { cancel as cancelOperation, runStreamed } from '@/api/queries/operations'
import type { CleanPlan, DomainAnswered } from '@/api/types.gen'
import { advance, type Progress, planned } from '@/features/worktrees/remove/utils/progress'

export type Start = { answers: DomainAnswered[]; fingerprint: string; plan: CleanPlan }

type Run = { progress: Progress; running: boolean }

// a store, not the dialog's state: the dialog shows the run again when it opens again
export const useRemoveRuns = create<Record<string, Run | undefined>>(() => ({}))

const update = (repo: string, change: (progress: Progress) => Progress) =>
  useRemoveRuns.setState((runs) => {
    const run = runs[repo]
    return run ? { [repo]: { ...run, progress: change(run.progress) } } : runs
  })

export function useRemoveRun(repo: string) {
  const client = useQueryClient()
  const run = useRemoveRuns((runs) => runs[repo])

  const start = async ({ answers, fingerprint, plan }: Start) => {
    useRemoveRuns.setState({ [repo]: { progress: planned(plan), running: true } })
    try {
      const envelope = await runStreamed({
        repo,
        command: 'clean',
        call: { answers, expect_fingerprint: fingerprint },
        started: (operation) => update(repo, (progress) => ({ ...progress, operation })),
        line: (line) => update(repo, (progress) => advance(progress, line)),
      })
      update(repo, (progress) => ({ ...progress, envelope }))
    } catch (error) {
      update(repo, (progress) => ({ ...progress, refused: error instanceof Error ? error.message : String(error) }))
    }
    useRemoveRuns.setState((runs) => ({ [repo]: runs[repo] && { ...runs[repo], running: false } }))
    await client.invalidateQueries({ queryKey: repoKey(repo) })
  }

  const cancel = (force: boolean) => {
    const operation = run?.progress.operation
    if (!operation) return
    update(repo, (progress) => ({ ...progress, cancelling: force ? 'forced' : 'asked' }))
    void cancelOperation(repo, operation, force)
  }

  return {
    progress: run?.progress,
    running: run?.running ?? false,
    start: (params: Start) => void start(params),
    cancel,
    dismiss: () => useRemoveRuns.setState({ [repo]: undefined }),
  }
}
