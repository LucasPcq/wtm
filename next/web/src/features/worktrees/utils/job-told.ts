import type { Applied } from '@/api/queries/commands'
import type { RunjobsJobResult } from '@/api/types.gen'
import type { Told } from '@/features/worktrees/hooks/use-direct-run'
import { sentence } from '@/features/worktrees/utils/sentence'
import { plural } from '@/lib/plural'

export type JobAt = { worktree: string; job: string }

export const holdOf = ({ worktree, job }: JobAt) => `${worktree}’s hold on ${job}`

// the cause: the item's error, else the job's own message, else why the run had no envelope
const failure = (...causes: (string | undefined)[]) => {
  const message = causes.find(Boolean)
  return message ? sentence(message) : undefined
}

const upAgain = (job: RunjobsJobResult | undefined) => {
  const port = job?.ports?.[0]?.port
  return port === undefined ? 'up again' : `up again on :${port}`
}

// a start of the job, again after a restart or an undo; a task's start is done once it ended
export function startedTold(
  { at, restart }: { at: JobAt; restart: boolean },
  envelope: Applied<'run-start'> | undefined,
  refused: string | undefined,
): Told {
  const result = envelope?.items[0]?.detail?.jobs?.find((one) => one.name === at.job)
  if (!envelope?.ok || !result || result.status === 'failed' || result.status === 'crashed')
    return {
      tone: 'error',
      title: restart ? `${at.job} didn’t restart` : `${at.job} didn’t start`,
      description: failure(envelope?.items[0]?.error?.message, refused),
    }
  if (result.status === 'done') return { tone: 'success', title: `${at.job} ran`, description: at.worktree }
  if (result.status === 'joined') return { tone: 'success', title: `${at.worktree} holds ${at.job} again` }
  return {
    tone: 'success',
    title: restart ? `${at.job} restarted` : `${at.job} started`,
    description: `${at.worktree} · ${upAgain(result)}`,
  }
}

// a stop of the job, or a release of the worktree's hold on a shared service; undo starts it again
export function stoppedTold(
  at: JobAt,
  envelope: Applied<'run-stop'> | undefined,
  refused: string | undefined,
  undo: Told['cancel'],
): Told {
  const result = envelope?.items[0]?.detail?.jobs?.find((one) => one.name === at.job)
  if (!envelope?.ok || !result || result.status === 'failed' || result.status === 'not_reached')
    return {
      tone: 'error',
      title: `${at.job} didn’t stop`,
      description: failure(envelope?.items[0]?.error?.message, result?.message, refused),
    }
  if (result.status === 'released') {
    const kept = envelope.plan?.worktrees?.[0]?.jobs?.[0]?.kept_by ?? []
    return {
      tone: 'success',
      title: `Released ${holdOf(at)}`,
      description: `It keeps running for ${plural(kept.length, 'worktree')}.`,
      cancel: undo,
    }
  }
  return {
    tone: 'success',
    title: `${at.job} stopped`,
    description: `${at.worktree} · its log stays here`,
    cancel: undo,
  }
}
