import { useQueryClient } from '@tanstack/react-query'
import { create } from 'zustand'
import { repoKey } from '@/api/queries/commands'
import { cancel as cancelOperation, runStreamed } from '@/api/queries/operations'
import type { CreatePlan, DomainAnswered } from '@/api/types.gen'
import { advance, type Progress, planned } from '@/features/worktrees/create/progress'

export type Start = { answers: DomainAnswered[]; fingerprint: string; plan: CreatePlan }

type Run = { progress: Progress; running: boolean }

// The create each repository runs from this page: the dialog shows it, closed and opened again, and the list marks
// the rows it works on.
export const useCreateRuns = create<Record<string, Run | undefined>>(() => ({}))

const update = (repo: string, change: (progress: Progress) => Progress) =>
  useCreateRuns.setState((runs) => {
    const run = runs[repo]
    return run ? { [repo]: { ...run, progress: change(run.progress) } } : runs
  })

export function useCreateRun(repo: string) {
  const client = useQueryClient()
  const run = useCreateRuns((runs) => runs[repo])

  const start = async ({ answers, fingerprint, plan }: Start) => {
    useCreateRuns.setState({ [repo]: { progress: planned(plan), running: true } })
    try {
      const envelope = await runStreamed({
        repo,
        command: 'create',
        call: { answers, expect_fingerprint: fingerprint },
        started: (operation) => update(repo, (progress) => ({ ...progress, operation })),
        line: (line) => update(repo, (progress) => advance(progress, line)),
      })
      update(repo, (progress) => ({ ...progress, envelope }))
    } catch (error) {
      update(repo, (progress) => ({ ...progress, refused: error instanceof Error ? error.message : String(error) }))
    }
    useCreateRuns.setState((runs) => ({ [repo]: runs[repo] && { ...runs[repo], running: false } }))
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
    dismiss: () => useCreateRuns.setState({ [repo]: undefined }),
  }
}

export function useUnderWay(repo: string): Record<string, string> {
  const run = useCreateRuns((runs) => runs[repo])
  if (!run?.running) return {}

  return Object.fromEntries(
    run.progress.units
      .filter((unit) => !unit.status)
      .map((unit) => {
        const done = unit.phases.filter((phase) => phase.state === 'done').length
        return [unit.subject, `Creating · ${done}/${unit.phases.length}`]
      }),
  )
}
