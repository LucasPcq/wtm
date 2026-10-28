import { useQuery, useQueryClient } from '@tanstack/react-query'
import { create } from 'zustand'
import { type Applied, type MutationCommand, repoKey } from '@/api/queries/commands'
import { current, eventsQuery } from '@/api/queries/events'
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

// an item the plan left as it is was never asked: a run that did all it was asked succeeded
export const succeeded = (envelope: Applied<MutationCommand> | undefined) =>
  envelope?.ok === true &&
  envelope.items.every((item) => item.status === 'done' || (item.status === 'skipped' && item.reason === 'ruled_out'))

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

  // a run taken before its plan is known, read from the store and not from the last render: a click that finds one
  // under way, still planning, or failed and not yet read, starts no second and gets that run's subjects; a run
  // cancelled was read in the toast that said so
  const claim = (units: Unit[]): string[] | undefined => {
    dismissSucceeded(repo, command)
    const held = useRuns.getState()[key]
    const told = held?.running === false && held.progress.envelope?.error?.kind === 'cancelled'
    if (held && !told) return held.progress.units.map((unit) => unit.subject)
    useRuns.setState({ [key]: { progress: { units }, running: true } })
    return undefined
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
    claim,
    cancel,
    dismiss: () => useRuns.setState({ [key]: undefined }),
  }
}

export const clockEvery = 30_000

const doing: Record<MutationCommand, string> = {
  create: 'Creating',
  clean: 'Removing',
  'fast-forward': 'Fast-forwarding',
  prune: 'Pruning',
}

export const isMutation = (command: string): command is MutationCommand => Object.hasOwn(doing, command)

export function useUnderWay(repo: string): Record<string, string> {
  const runs = useRuns()
  const operations = useQuery({ ...eventsQuery, select: (live) => live.operations }).data ?? {}
  // the cap needs a clock: an operation whose process died sends nothing more, so the screen asks the time again
  const now =
    useQuery({
      queryKey: ['now'],
      queryFn: () => Date.now(),
      refetchInterval: clockEvery,
      enabled: current(operations, Date.now()).length > 0,
    }).data ?? Date.now()

  const elsewhere = current(operations, now).flatMap((operation) =>
    operation.repo === repo
      ? operation.subjects.map((subject) => [
          subject,
          isMutation(operation.command) ? doing[operation.command] : 'Busy',
        ])
      : [],
  )
  const here = (Object.keys(doing) as MutationCommand[]).flatMap((command) => {
    const run = runs[keyOf(repo, command)]
    if (!run?.running) return []
    return run.progress.units
      .filter((unit) => !unit.status)
      .map((unit) => {
        const done = unit.phases.filter((phase) => phase.state === 'done').length
        // a count only for several: one step has nothing to count
        return [
          unit.subject,
          unit.phases.length > 1 ? `${doing[command]} · ${done}/${unit.phases.length}` : doing[command],
        ]
      })
  })

  return Object.fromEntries([...elsewhere, ...here])
}
