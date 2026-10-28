import type { Applied } from '@/api/queries/commands'
import type { DomainError, ReparentMove } from '@/api/types.gen'
import type { Told } from '@/features/worktrees/hooks/use-direct-run'
import { moved } from '@/features/worktrees/reparent/utils/plan'
import { listed } from '@/features/worktrees/utils/listed'
import type { OutcomeKind } from '@/features/worktrees/utils/outcome'
import { isRefusal, refusedWhy } from '@/features/worktrees/utils/reparent'
import { sentence } from '@/features/worktrees/utils/sentence'

export const title = 'Change parent'

export const outcomeWords: Record<OutcomeKind, string> = {
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

// the toasts of worktrees reparented at once, one run per new parent
export function toldOf(envelopes: Applied<'reparent'>[], refused?: string): Omit<Told, 'action' | 'cancel'> {
  const last = envelopes.at(-1)
  const went = moved(envelopes)
  const parents = [...new Set(went.flatMap((item) => (item.detail ? [item.detail.to] : [])))]
  const [only] = went
  const failed = last?.items.find((item) => item.status === 'failed')

  if (last?.error?.kind === 'cancelled' || last?.items.some((item) => item.reason === 'interrupted'))
    return { tone: 'info', title: 'Reparent cancelled', description: `${went.length} moved before it stopped` }
  if (!last || failed || !last.ok) {
    const subjects = (last?.items ?? []).filter((item) => item.status !== 'done').map((item) => item.subject)
    return {
      tone: 'error',
      title: `Couldn't reparent ${listed(subjects) || 'the worktrees'}`,
      description: failureOf(failed?.error ?? last?.error) ?? refused ?? 'It stopped before it ran',
    }
  }

  if (went.length === 1 && only?.detail)
    return {
      tone: 'success',
      title: `${only.subject} is on ${only.detail.to}`,
      description: 'It rebases onto it on its next sync.',
    }
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
