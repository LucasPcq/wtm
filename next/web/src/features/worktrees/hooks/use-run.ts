import { useQueryClient } from '@tanstack/react-query'
import { create } from 'zustand'
import { type MutationCommand, repoKey } from '@/api/queries/commands'
import { cancel as cancelOperation, runStreamed } from '@/api/queries/operations'
import type { DomainAnswered } from '@/api/types.gen'
import { advance, type Progress, type Unit } from '@/features/worktrees/utils/progress'

export type Start = { answers: DomainAnswered[]; fingerprint: string; units: Unit[] }

type Run = { progress: Progress<MutationCommand>; running: boolean }

// a store, not a component's state: the dialog shows the run again when it opens again, and the list marks its rows
export const useRuns = create<Record<string, Run | undefined>>(() => ({}))

const keyOf = (repo: string, command: MutationCommand) => `${command}:${repo}`

export function useRun<C extends MutationCommand>(repo: string, command: C) {
  const client = useQueryClient()
  const key = keyOf(repo, command)
  const run = useRuns((runs) => runs[key])

  const update = (change: (progress: Progress<C>) => Progress<C>) =>
    useRuns.setState((runs) => {
      const now = runs[key]
      return now ? { [key]: { ...now, progress: change(now.progress as Progress<C>) } } : runs
    })

  const start = async ({ answers, fingerprint, units }: Start) => {
    useRuns.setState({ [key]: { progress: { units }, running: true } })
    try {
      const envelope = await runStreamed({
        repo,
        command,
        call: { answers, expect_fingerprint: fingerprint },
        started: (operation) => update((progress) => ({ ...progress, operation })),
        line: (line) => update((progress) => advance(progress, line)),
      })
      update((progress) => ({ ...progress, envelope }))
    } catch (error) {
      update((progress) => ({ ...progress, refused: error instanceof Error ? error.message : String(error) }))
    }
    useRuns.setState((runs) => ({ [key]: runs[key] && { ...runs[key], running: false } }))
    await client.invalidateQueries({ queryKey: repoKey(repo) })
  }

  const cancel = (force: boolean) => {
    const operation = run?.progress.operation
    if (!operation) return
    update((progress) => ({ ...progress, cancelling: force ? 'forced' : 'asked' }))
    void cancelOperation(repo, operation, force)
  }

  return {
    progress: run?.progress as Progress<C> | undefined,
    running: run?.running ?? false,
    start: (params: Start) => void start(params),
    cancel,
    dismiss: () => useRuns.setState({ [key]: undefined }),
  }
}

const doing: Record<MutationCommand, string> = { create: 'Creating', clean: 'Removing' }

// the rows a run works on, each with what it is doing
export function useUnderWay(repo: string): Record<string, string> {
  const runs = useRuns()

  return Object.fromEntries(
    (Object.keys(doing) as MutationCommand[]).flatMap((command) => {
      const run = runs[keyOf(repo, command)]
      if (!run?.running) return []
      return run.progress.units
        .filter((unit) => !unit.status)
        .map((unit) => {
          const done = unit.phases.filter((phase) => phase.state === 'done').length
          return [unit.subject, `${doing[command]} · ${done}/${unit.phases.length}`]
        })
    }),
  )
}
