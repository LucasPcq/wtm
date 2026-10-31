import type { SyncBlocker, SyncRebase, SyncStanding } from '@/api/types.gen'
import { operationWords } from '@/features/worktrees/utils/operation'
import type { OutcomeLabels, OutcomeTone } from '@/features/worktrees/utils/outcome'
import { plural } from '@/lib/plural'

export const commits = (n: number) => plural(n, 'commit')

export const changes = (n: number | undefined) =>
  n ? `${n} uncommitted ${n === 1 ? 'change' : 'changes'}` : 'uncommitted changes'

export const names = (list: string[]) =>
  list.length < 2 ? (list[0] ?? '') : `${list.slice(0, -1).join(', ')} and ${list.at(-1)}`

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
  'sync.push_failed': () => 'git refused the push; its rebase stands',
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

export const outcomeWords: OutcomeLabels = {
  failed: "Couldn't sync",
  check: 'To check',
  done: 'Synced',
  along: 'Fast-forwarded',
}

export type Switch = 'push' | 'keep_conflict'

export const switchWords: Record<Switch, { title: string; on: string; off: string }> = {
  push: {
    title: 'Push to origin',
    on: 'Each is pushed after its rebase, with force-with-lease: refused if origin moved since.',
    off: 'Each stays local; origin keeps its old tips until you push.',
  },
  keep_conflict: {
    title: 'Leave a conflict to resolve',
    on: 'A conflict stays in progress for you to finish in its worktree.',
    off: 'A conflict stops that rebase and leaves the worktree as it was.',
  },
}

export const sentenceWords = {
  pushed: 'Pushed after each rebase, with force-with-lease: refused if origin moved since.',
  pushOnly: 'Pushed with force-with-lease: refused if origin moved since.',
  notPushed: 'Not pushed: origin keeps the old tips until you push.',
  kept: 'A conflict stays in progress for you to finish in its worktree; the ones under it wait.',
  stopped: 'A conflict stops that rebase and leaves the worktree as it was; the ones under it wait.',
  asItStands: (onto: string[]) =>
    `${names(onto)} ${onto.length === 1 ? "isn't" : "aren't"} fast-forwarded first: each rebases onto it as it stands.`,
}
