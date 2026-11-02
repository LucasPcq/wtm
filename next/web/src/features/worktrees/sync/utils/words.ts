import type { SyncBlocker, SyncRebase, SyncStanding } from '@/api/types.gen'
import { changes, commits } from '@/features/worktrees/utils/counts'
import { operationWords } from '@/features/worktrees/utils/operation'
import type { OutcomeKind, OutcomeTone } from '@/features/worktrees/utils/outcome'
import { plural } from '@/lib/plural'

// the core words its standings for the command line; the web says each its own way, so a standing it adds fails the
// build until it has words
export const standingWords: Record<SyncStanding, (rebase: SyncRebase) => string> = {
  missing: () => 'its folder is gone',
  busy: ({ operation }) => `${operation ? operationWords[operation] : 'mid-operation'}: finish it in its worktree`,
  unreadable: () => "its record doesn't read",
  in_cycle: () => 'its parents lead back to it',
  no_parent: () => 'no parent recorded',
  parent_gone: ({ parent }) => `its parent ${parent} is no branch any more`,
  unrelated: ({ parent }) => `shares no commit with its parent ${parent}`,
  parent_left: ({ parent }) => `its parent ${parent} is left as it is`,
  diverged: ({ origin }) => `diverged from origin: ${origin?.ahead ?? 0} ahead, ${origin?.behind ?? 0} behind`,
  rebases: ({ parent, commits: own }) => `onto ${parent} · ${commits(own)}`,
  on_parent: ({ parent }) => `already on ${parent}`,
}

// a business state the person may act on is amber; one there is nothing to do about, grey
export const standingTone: Record<SyncStanding, OutcomeTone> = {
  missing: 'attention',
  busy: 'attention',
  unreadable: 'attention',
  in_cycle: 'attention',
  no_parent: 'muted',
  parent_gone: 'attention',
  unrelated: 'attention',
  parent_left: 'muted',
  diverged: 'attention',
  rebases: 'muted',
  on_parent: 'muted',
}

// what a worktree of the plan does, from its own phases: a rebase onto its parent, else its own origin's commits
// taken, else a push alone
export function phaseWords(rebase: SyncRebase, onto = rebase.parent): string {
  const phases = rebase.phases ?? []
  if (phases.includes('rebase')) return `onto ${onto} · ${commits(rebase.commits)}`
  if (phases.includes('fast_forward')) return `fast-forwarded to origin · ${commits(rebase.origin?.behind ?? 0)}`
  if (phases.includes('push')) return 'pushed to origin'
  return standingWords[rebase.standing](rebase)
}

export const blockerWords: Record<SyncBlocker, (count: number | undefined) => string> = {
  dirty: (count) => changes(count),
}

// the codes an item fails on, as the screen says each on its folded line; git's own words show once it opens
type Failure = 'sync.conflict' | 'sync.failed' | 'sync.dirty' | 'sync.busy' | 'sync.push_failed' | 'sync.ff_failed'

export const failureWords: Record<Failure, (params: Record<string, string>) => string> = {
  'sync.conflict': ({ parent, kept }) =>
    kept === 'true' ? `conflicts with ${parent}: left to resolve` : `conflicts with ${parent}; nothing changed`,
  'sync.failed': () => 'git refused it; nothing changed',
  'sync.dirty': () => 'it gained uncommitted changes since the plan',
  'sync.busy': () => 'an operation began in it since the plan',
  'sync.push_failed': () => 'rebased, but origin refused the push',
  'sync.ff_failed': ({ children }) => `couldn't take origin's commits: ${children} rebased onto it as it stands`,
}

export const isFailure = (code: string): code is Failure => Object.hasOwn(failureWords, code)

// what to do once a failed row opens, in the person's words
export const failureHelp: Record<Failure, (params: Record<string, string>) => string> = {
  'sync.conflict': ({ kept }) =>
    kept === 'true'
      ? 'The rebase stays in progress in its worktree: resolve the files, run git rebase --continue there, then sync the rest.'
      : 'Its rebase was stopped and the worktree left as it was. Run it again keeping the conflict to resolve it by hand.',
  'sync.failed': () => 'Nothing moved in this worktree.',
  'sync.dirty': () => 'It is left as it is; the plan saw it clean.',
  'sync.busy': () => 'It is left as it is: finish that operation first.',
  'sync.push_failed': () => 'Its rebase stands; origin moved since the plan, so the push was refused.',
  'sync.ff_failed': () => 'The worktrees on it were rebased onto it as it stood.',
}

export const outcomeWords: Record<OutcomeKind, string> = {
  failed: "Couldn't sync",
  check: 'To check',
  done: 'Synced',
}

export type Switch = 'push' | 'keep_conflict'

export const switchWords: Record<Switch, { title: string; on: string; off: string }> = {
  push: {
    title: 'Push to origin',
    on: 'Each rebase is pushed; refused if origin moved since.',
    off: 'Nothing is pushed: origin keeps the old tips.',
  },
  keep_conflict: {
    title: 'Leave a conflict to resolve',
    on: 'A conflict stays in progress, to resolve in its worktree.',
    off: 'A conflict cancels that rebase; its worktree is untouched.',
  },
}

// one line each at the dialog's width, whatever the number of worktrees: a list of names is a count past one
const subject = (names: string[], many: 'worktrees' | 'branches') =>
  names.length === 1 ? (names[0] ?? '') : `${names.length} ${many}`

export const sentenceWords = {
  pushed: 'Each rebase is pushed; refused if origin moved since.',
  pushOnly: 'Each is pushed to origin; refused if origin moved since.',
  forwardOnly: (branches: string[]) =>
    `${subject(branches, 'branches')} ${branches.length === 1 ? 'takes' : 'take'} origin's commits; nothing is rebased.`,
  notPushed: 'Not pushed: origin keeps the old tips.',
  kept: 'A conflict stays to resolve; the ones under it wait.',
  stopped: 'A conflict cancels that rebase; the ones under it wait.',
  asItStands: (onto: string[]) =>
    `${subject(onto, 'branches')} not fast-forwarded first: rebased onto as ${onto.length === 1 ? 'it stands' : 'they stand'}.`,
  // the worktrees under the ones picked, counted (each row names its own): those that follow, those kept for their changes
  descendants: (follow: string[], kept: string[], root?: string) =>
    [
      follow.length > 0 &&
        `${plural(follow.length, 'worktree')} ${follow.length === 1 ? 'follows' : 'follow'}${root ? ` ${root}` : ''}`,
      kept.length > 0 &&
        `${follow.length > 0 ? kept.length : plural(kept.length, 'worktree')} kept: uncommitted changes`,
    ]
      .filter(Boolean)
      .join(' · '),
  leftOut: (left: string[]) =>
    `${subject(left, 'worktrees')} ${left.length === 1 ? 'stays as it is; sync it' : 'stay as they are; sync them'} after.`,
}
