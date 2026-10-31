import type {
  DomainOption,
  FastforwardAdvance,
  FastforwardBlocker,
  FastforwardStanding,
  WorktreeDivergence,
} from '@/api/types.gen'
import { changes } from '@/features/worktrees/utils/counts'
import { operationWords } from '@/features/worktrees/utils/operation'
import type { OutcomeKind, OutcomeTone } from '@/features/worktrees/utils/outcome'

// the core words its standings for the command line; the web says each its own way, so a standing it adds fails the
// build until it has words
export const standingWords: Record<FastforwardStanding, (advance: FastforwardAdvance) => string> = {
  missing: () => 'its folder is gone',
  busy: ({ operation }) => `${operation ? operationWords[operation] : 'mid-operation'}: finish it in its worktree`,
  no_origin: () => 'not on origin',
  diverged: ({ origin }) => `diverged from origin: ${origin?.ahead ?? 0} ahead, ${origin?.behind ?? 0} behind`,
  ahead: ({ origin }) => `${origin?.ahead ?? 0} ahead of origin`,
  up_to_date: () => 'up to date with origin',
  behind: ({ origin }) => `${origin?.behind ?? 0} behind origin`,
}

// a business state the person may act on is amber; one there is nothing to do about, grey
export const standingTone: Record<FastforwardStanding, OutcomeTone> = {
  missing: 'attention',
  busy: 'attention',
  no_origin: 'muted',
  diverged: 'attention',
  ahead: 'muted',
  up_to_date: 'muted',
  behind: 'attention',
}

export const blockerWords: Record<FastforwardBlocker, string> = { dirty: 'uncommitted changes' }

export const keptWords = (blockers: FastforwardBlocker[]) =>
  `${blockers.map((blocker) => blockerWords[blocker]).join(', ')}: kept as it is`

// the picker's words, from where each option stands as origin was last fetched
const divergenceWords: Record<WorktreeDivergence, (option: DomainOption) => string> = {
  behind: ({ origin }) => `${origin?.behind ?? 0} behind origin`,
  ahead: ({ origin }) => `${origin?.ahead ?? 0} ahead of origin`,
  diverged: ({ origin }) => `diverged: ${origin?.ahead ?? 0} ahead, ${origin?.behind ?? 0} behind`,
  'up-to-date': () => 'up to date with origin',
}

export function optionWords(option: DomainOption): { attention: string[]; muted: string[] } {
  const standing = option.origin ? divergenceWords[option.origin.state](option) : 'not on origin'
  const dirty = (option.changes ?? 0) > 0 ? [changes(option.changes ?? 0)] : []
  if (option.origin?.state === 'diverged') return { attention: [...dirty, standing], muted: [] }
  return { attention: dirty, muted: [standing] }
}

// the fixed line of each failure the core codes, as the screen says it; git's own words show once the row opens
export const failureWords: Record<string, string> = {
  'fast-forward.failed': 'git refused to move the branch',
  'fast-forward.dirty': 'it gained uncommitted changes since the plan',
  'fast-forward.busy': 'an operation began in it since the plan',
}

export const outcomeWords: Record<OutcomeKind, string> = {
  failed: "Couldn't fast-forward",
  check: 'To check',
  done: 'Fast-forwarded',
}
