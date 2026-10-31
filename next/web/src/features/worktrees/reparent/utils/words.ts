import type { Applied } from '@/api/queries/commands'
import type { DomainError, ReparentMove } from '@/api/types.gen'
import type { Told } from '@/features/worktrees/hooks/use-direct-run'
import { moved } from '@/features/worktrees/reparent/utils/plan'
import { listed } from '@/features/worktrees/utils/listed'
import type { OutcomeLabels } from '@/features/worktrees/utils/outcome'
import { isRefusal, refusedWhy } from '@/features/worktrees/utils/reparent'
import { sentence } from '@/features/worktrees/utils/sentence'

export const title = 'Change parent'

export const outcomeWords: OutcomeLabels = {
  failed: "Couldn't reparent",
  check: 'To check',
  done: 'Reparented',
}

export const parentWords = ({ from, parent_gone }: Pick<ReparentMove, 'from' | 'parent_gone'>) => {
  if (from === '') return 'no parent'
  return parent_gone ? `${from} (deleted)` : from
}

export const moveWords = (move: Pick<ReparentMove, 'from' | 'parent_gone' | 'to'>) =>
  `${parentWords(move)} → ${move.to}`

export const followWords = (children: string[]) =>
  children.length === 0 ? undefined : `${listed(children)} ${children.length === 1 ? 'follows' : 'follow'}`

// a refusal names the branch it refuses: "feat/billing-ui sits under feat/billing"
export function refusalOf(error: DomainError | undefined) {
  const [field] = error?.fields ?? []
  if (field && isRefusal(field.code)) return `${field.params?.value} ${refusedWhy(field)}`
  if (field) return sentence(field.message)
  return error ? sentence(error.message) : 'It stopped before it ran'
}

// the folded line is the code's fixed words; its cause, verbatim, is in the fold
const failureWords: Record<string, string> = {
  'reparent.write_failed': "couldn't write its record",
}

export const failureOf = (error: DomainError | undefined) => {
  if (!error) return undefined
  return failureWords[error.code] ?? sentence(error.message)
}

// what moved, said in one line: "a and c moved onto main"
const movedWords = (went: ReturnType<typeof moved>) => {
  const parents = [...new Set(went.flatMap((item) => (item.detail ? [item.detail.to] : [])))]
  const names = listed(went.map((item) => item.subject))
  return parents.length === 1 ? `${names} moved onto ${parents[0]}` : `${names} moved`
}

// the toast of worktrees reparented at once, one run per new parent: envelopes are the runs that ended, the last this
// one unless it was refused before any envelope (refused); worktrees are its own
export function toldOf(
  envelopes: Applied<'reparent'>[],
  { worktrees, refused }: { worktrees: string[]; refused?: string | undefined },
): Omit<Told, 'action' | 'cancel'> {
  const last = refused === undefined ? envelopes.at(-1) : undefined
  const went = moved(envelopes)
  const failed = last?.items.find((item) => item.status === 'failed')

  if (last?.error?.kind === 'cancelled' || last?.items.some((item) => item.reason === 'interrupted'))
    return { tone: 'info', title: 'Reparent cancelled', description: `${went.length} moved before it stopped` }
  if (!last || failed || !last.ok) {
    const stuck = (last?.items ?? []).filter((item) => item.status !== 'done').map((item) => item.subject)
    const why = failureOf(failed?.error ?? last?.error) ?? refused ?? 'It stopped before it ran'
    return {
      tone: 'error',
      title: `Couldn't reparent ${listed(stuck.length > 0 ? stuck : worktrees)}`,
      description: went.length > 0 ? `${sentence(why)}. ${movedWords(went)}.` : sentence(why),
    }
  }

  const [only] = went
  if (went.length === 1 && only?.detail)
    return {
      tone: 'success',
      title: `${only.subject} is on ${only.detail.to}`,
      description: 'It rebases onto it on its next sync.',
    }
  const parents = [...new Set(went.flatMap((item) => (item.detail ? [item.detail.to] : [])))]
  const [parent] = parents
  if (parents.length === 1)
    return {
      tone: 'success',
      title: `${went.length} worktrees moved onto ${parent}`,
      description: `Each rebases onto ${parent} on its next sync.`,
    }
  return {
    tone: 'success',
    title: `${went.length} worktrees moved`,
    description: 'Each rebases onto its new parent on its next sync.',
  }
}
