import { useQueryClient } from '@tanstack/react-query'
import { create } from 'zustand'
import { type Applied, type MutationCommand, repoKey } from '@/api/queries/commands'
import { cancel as cancelOperation, runStreamed } from '@/api/queries/operations'
import type { DomainAnswered } from '@/api/types.gen'
import { advance, type Progress, type Unit } from '@/features/worktrees/utils/progress'

export type Start = { answers: DomainAnswered[]; fingerprint: string; units: Unit[] }

type Run = { progress: Progress<MutationCommand>; running: boolean }

// a store, not a component's state: the dialog shows the run again when it opens again, and the list marks its rows
export const useRuns = create<Record<string, Run | undefined>>(() => ({}))

const keyOf = (repo: string, command: MutationCommand) => `${command}:${repo}`

// a run that ended well has nothing left to read: the next one opens on its own subjects; one still running, or one
// that failed even in part, stays until it is read, one at a time
export function dismissSucceeded(repo: string, command: MutationCommand) {
  const key = keyOf(repo, command)
  const run = useRuns.getState()[key]
  if (run?.running === false && succeeded(run.progress.envelope)) useRuns.setState({ [key]: undefined })
}

export const succeeded = (envelope: Applied<MutationCommand> | undefined) =>
  envelope?.ok === true && envelope.items.every((item) => item.status === 'done')

export function useRun<C extends MutationCommand>(repo: string, command: C) {
  const client = useQueryClient()
  const key = keyOf(repo, command)
  const run = useRuns((runs) => runs[key])

  const update = (change: (progress: Progress<C>) => Progress<C>) =>
    useRuns.setState((runs) => {
      const now = runs[key]
      return now ? { [key]: { ...now, progress: change(now.progress as Progress<C>) } } : runs
    })

  const answer = async ({ answers, fingerprint }: Start): Promise<Applied<C> | undefined> => {
    try {
      const envelope = await runStreamed({
        repo,
        command,
        call: { answers, expect_fingerprint: fingerprint },
        started: (operation) => update((progress) => ({ ...progress, operation })),
        line: (line) => update((progress) => advance(progress, line)),
      })
      update((progress) => ({ ...progress, envelope }))
      return envelope
    } catch (error) {
      update((progress) => ({ ...progress, refused: error instanceof Error ? error.message : String(error) }))
      return undefined
    }
  }

  const start = async (params: Start, ended?: (envelope: Applied<C> | undefined) => void) => {
    useRuns.setState({ [key]: { progress: { units: params.units }, running: true } })
    const envelope = await answer(params)
    useRuns.setState((runs) => ({ [key]: runs[key] && { ...runs[key], running: false } }))
    ended?.(envelope)
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
    start: (params: Start, ended?: (envelope: Applied<C> | undefined) => void) => void start(params, ended),
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
