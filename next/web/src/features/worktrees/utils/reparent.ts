import type { DomainAnswered, DomainFieldError } from '@/api/types.gen'
import { sentence } from '@/features/worktrees/utils/sentence'

export const requestOf = ({ worktrees, to }: { worktrees: string[]; to: string }): DomainAnswered[] => [
  { path: 'worktree', values: worktrees },
  { path: 'to', values: [to] },
]

// the new parents reparent refuses for the worktrees it moves, as the picker words them beside the branch
type Refusal = 'self_parent' | 'reparent.cycle' | 'not_found'

const refusalWords: Record<Refusal, (params: Record<string, string>) => string> = {
  self_parent: () => "the worktree you're moving",
  not_found: () => 'no longer exists',
  'reparent.cycle': (params) => `sits under ${params.under}`,
}

export const isRefusal = (code: string): code is Refusal => Object.hasOwn(refusalWords, code)

export const refusedWhy = (error: DomainFieldError) =>
  isRefusal(error.code) ? refusalWords[error.code](error.params ?? {}) : sentence(error.message)
