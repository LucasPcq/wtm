import type { Applied } from '@/api/queries/commands'
import type { DomainAnswered, DomainFollowUp, DomainPhase, DomainWarning, SyncPlan, SyncRebase } from '@/api/types.gen'
import { moves, unsafe } from '@/features/worktrees/sync/utils/plan'
import {
  failureHelp,
  failureWords,
  isFailure,
  phaseWords,
  standingTone,
  standingWords,
} from '@/features/worktrees/sync/utils/words'
import { subjectsOf } from '@/features/worktrees/utils/actions'
import { changes, commits } from '@/features/worktrees/utils/counts'
import { type OutcomeGroup, type OutcomeRow, outputOf } from '@/features/worktrees/utils/outcome'
import type { Progress } from '@/features/worktrees/utils/progress'
import { sentence } from '@/features/worktrees/utils/sentence'

// the fixes a sync warning offers, each a sync the dialog plans again
type Item = Applied<'sync'>['items'][number]

const followUpWords: Record<string, string> = { 'sync.force': 'Sync anyway', 'sync.parents': 'Fast-forward first' }

const reparentLabel = (followUp: DomainFollowUp) => {
  const to = followUp.answers?.find((answer) => answer.path === 'to')?.values[0]
  return to ? `Reparent onto ${to}` : 'Reparent'
}

// the worktrees the plan leaves as they are, each with why and its fix: a reparent runs in the dialog, anything else
// is copied
export function leftRowsOf({
  plan,
  review,
  warnings,
  changesOf,
  onReparent,
}: {
  plan: SyncPlan
  review: SyncRebase[]
  warnings: DomainWarning[]
  changesOf: (worktree: string) => number | undefined
  onReparent: (followUp: DomainFollowUp) => void
}): OutcomeRow[] {
  const inPlan = (worktree: string) => (plan.worktrees ?? []).some((rebase) => rebase.worktree === worktree)
  // a worktree step 1 showed as one it cannot rebase (mid-rebase, folder gone, diverged…) is left all the same when the
  // command's own pick never named it
  const kept = [
    ...(plan.worktrees ?? []).filter((rebase) => !moves(rebase)),
    ...review.filter(
      (rebase) => !inPlan(rebase.worktree) && !['rebases', 'on_parent', 'in_cycle'].includes(rebase.standing),
    ),
  ]
  return [
    ...kept.map((rebase): OutcomeRow => {
      const followUp = warnings.find((warning) => warning.params?.worktree === rebase.worktree)?.follow_up
      const reparent = followUp?.command === 'reparent' ? followUp : undefined
      return {
        key: rebase.worktree,
        tone: unsafe(rebase) ? 'attention' : standingTone[rebase.standing],
        title: rebase.worktree,
        message: unsafe(rebase)
          ? `${changes(changesOf(rebase.worktree))}: kept · needs a sync after`
          : standingWords[rebase.standing](rebase),
        // a dirty one kept is checked on step 1, never forced from here
        followUp: unsafe(rebase) || reparent ? undefined : followUp,
        act: reparent && (() => onReparent(reparent)),
        actLabel: reparent && reparentLabel(reparent),
      }
    }),
    ...(plan.left_out ?? []).map((worktree) => {
      const parent = review.find((rebase) => rebase.worktree === worktree)?.parent
      return {
        key: worktree,
        tone: 'attention' as const,
        title: worktree,
        message: `stays on ${parent ? `${parent}'s old tip` : 'its parent'} · needs a sync after`,
      }
    }),
  ]
}

// what a run did, in the house's groups: what couldn't sync or push (with what stopped under it), what to check, the
// worktrees synced, then the branches fast-forwarded first; a follow-up the web runs is a button on the row it serves
export function recapOf(
  progress: Progress<'sync'>,
  run: (answers: DomainAnswered[]) => void,
): { groups: OutcomeGroup[]; warnings: DomainWarning[]; summary: string } {
  const envelope = progress.envelope
  const plan = envelope?.plan
  const items = envelope?.items ?? []
  const warnings = envelope?.warnings ?? []
  const followUps = envelope?.follow_ups ?? []
  const onto = (plan?.onto ?? []).map((branch) => branch.branch)
  const planned = (worktree: string) => plan?.worktrees?.find((rebase) => rebase.worktree === worktree)
  const has = (worktree: string, phase: DomainPhase) => (planned(worktree)?.phases ?? []).includes(phase)
  const followUpOf = (code: string, subject?: string) =>
    followUps.find((followUp) => followUp.code === code && (!subject || subjectsOf(followUp).includes(subject)))
  const pushRefused = (item: Item | undefined) => item?.status === 'failed' && item.error?.code === 'sync.push_failed'
  const itemOf = (subject: string) => items.find((item) => item.subject === subject)

  const failedRow = (item: Item): OutcomeRow => {
    const error = item.error
    const params = error?.params ?? {}
    const resolve = followUpOf('sync.resolve', item.subject)
    const resume = followUpOf('sync.resume', item.subject)
    const again = resolve ?? resume
    const lines = outputOf(progress.units.find((unit) => unit.subject === item.subject))?.lines
    // git's own lines when the run kept them, else its cause, else the files in conflict
    const output = () => {
      if (lines) return { lines }
      if (params.cause) return { lines: params.cause.split('\n') }
      if (params.files) return { lines: params.files.split(', ').map((file) => `conflict in ${file}`) }
      return undefined
    }
    return {
      key: item.subject,
      tone: item.status === 'failed' ? 'failed' : 'attention',
      title: item.subject,
      message: error && (isFailure(error.code) ? failureWords[error.code](params) : sentence(error.message)),
      note: error && isFailure(error.code) ? failureHelp[error.code](params) : undefined,
      output: output(),
      go: followUpOf('sync.go', item.subject),
      act: again && (() => run(again.answers ?? [])),
      actLabel: resolve ? 'Run again, keep the conflict' : 'Sync the rest',
    }
  }

  const stoppedRow = (item: Item): OutcomeRow => {
    const parent = planned(item.subject)?.parent ?? ''
    const blocked = () => {
      if (pushRefused(itemOf(parent))) return `blocked: ${parent}'s push was refused`
      return `blocked: ${parent} didn't sync`
    }
    return {
      key: item.subject,
      tone: 'muted',
      title: item.subject,
      message: item.reason === 'blocked' ? blocked() : 'not started: cancelled first',
    }
  }

  // a warning whose fix is a sync is run here, never copied: sync runs in the web
  const syncing = warnings.filter((warning) => warning.follow_up?.command === 'sync')
  const push = followUpOf('sync.push')
  const unpushed = push ? subjectsOf(push) : []
  const leftOut = followUpOf('sync.left_out')
  const leftNames = leftOut ? subjectsOf(leftOut) : []
  const failed = items.filter(
    (item) =>
      !onto.includes(item.subject) &&
      (item.status === 'failed' ||
        item.status === 'cancelled' ||
        item.reason === 'blocked' ||
        item.reason === 'interrupted'),
  )
  const worktreesFailed = failed.filter((item) => item.status === 'failed')
  const done = items.filter((item) => item.status === 'done' && item.reason !== 'unchanged')

  return {
    groups: [
      {
        kind: 'failed',
        label: worktreesFailed.length > 0 && worktreesFailed.every(pushRefused) ? "Couldn't push" : undefined,
        rows: failed.map((item) => (item.status === 'failed' ? failedRow(item) : stoppedRow(item))),
      },
      {
        kind: 'check',
        // a branch rebased onto that could not be fast-forwarded is skipped, never failed: the run went on
        rows: [
          ...items.filter((item) => onto.includes(item.subject) && item.error).map(failedRow),
          ...syncing.map((warning) => ({
            key: `${warning.code}-${warning.message}`,
            tone: 'attention' as const,
            message: sentence(warning.message),
            act: () => run(warning.follow_up?.answers ?? []),
            actLabel: followUpWords[warning.follow_up?.code ?? ''] ?? 'Sync',
          })),
          ...(push
            ? [
                {
                  key: 'push',
                  tone: 'attention' as const,
                  message: `Not pushed: origin keeps the old ${unpushed.length === 1 ? 'tip' : 'tips'} until you push`,
                  act: () => run(push.answers ?? []),
                  actLabel: unpushed.length === 1 ? 'Push it' : 'Push them',
                },
              ]
            : []),
        ],
      },
      ...(leftOut
        ? [
            {
              kind: 'check' as const,
              label: 'Left as they are',
              action: { label: leftActionWords(leftNames.length), run: () => run(leftOut.answers ?? []) },
              rows: leftNames.map((worktree) => ({
                key: `left-${worktree}`,
                tone: 'attention' as const,
                title: worktree,
                message: 'needs a sync',
              })),
            },
          ]
        : []),
      {
        kind: 'done',
        rows: done
          .filter((item) => !onto.includes(item.subject))
          .map((item) => {
            const rebase = planned(item.subject)
            const where = rebase ? phaseWords(rebase) : ''
            return {
              key: item.subject,
              tone: 'ok' as const,
              title: item.subject,
              message: [where, item.detail?.pushed && has(item.subject, 'rebase') && 'pushed']
                .filter(Boolean)
                .join(' · '),
              path: item.detail?.path,
            }
          }),
      },
      {
        kind: 'done',
        label: 'Fast-forwarded',
        rows: done
          .filter((item) => onto.includes(item.subject))
          .map((item) => ({
            key: item.subject,
            tone: 'ok' as const,
            title: item.subject,
            message: `${commits(item.detail?.commits ?? 0)} from origin`,
            path: item.detail?.path || undefined,
          })),
      },
    ],
    warnings: warnings.filter((warning) => !syncing.includes(warning)),
    summary: summaryOf(progress),
  }
}

const leftActionWords = (count: number) => {
  if (count === 1) return 'Sync it'
  return count === 2 ? 'Sync both' : 'Sync them'
}

// the one sentence a run ends on, in the dialog and in the toast of a run hidden: the count rule counts rebases, and a
// push origin refused leaves its rebase standing
export function summaryOf(progress: Progress<'sync'>): string {
  const envelope = progress.envelope
  const plan = envelope?.plan
  const items = envelope?.items ?? []
  const rows = plan?.worktrees ?? []
  const phased = (phase: DomainPhase) => rows.filter((rebase) => (rebase.phases ?? []).includes(phase))
  const itemOf = (worktree: string) => items.find((item) => item.subject === worktree)
  const stands = (worktree: string) => {
    const item = itemOf(worktree)
    return item?.status === 'done' || (item?.status === 'failed' && item.error?.code === 'sync.push_failed')
  }
  const asked = phased('rebase').length
  const synced = phased('rebase').filter((rebase) => stands(rebase.worktree)).length
  const pushes = phased('push').length
  const pushed = items.filter((item) => item.detail?.pushed).length
  const forwarded = phased('fast_forward').filter((rebase) => itemOf(rebase.worktree)?.status === 'done').length
  const failed = progress.refused !== undefined || envelope?.ok === false
  const cancelled = envelope?.error?.kind === 'cancelled' || items.some((item) => item.status === 'cancelled')

  if (cancelled) return `Cancelled: ${synced} of ${asked} synced.`
  if (asked === 0 && pushes > 0) return failed ? `${pushed} of ${pushes} pushed.` : `${pushed} pushed.`
  if (asked === 0) return failed ? 'Nothing synced.' : `${forwarded} fast-forwarded.`
  if (failed && !(synced === asked && plan?.push)) return `${synced} of ${asked} synced.`
  if (plan?.push) return pushed === synced ? `${synced} synced and pushed.` : `${synced} synced, ${pushed} pushed.`
  return `${synced} synced.`
}
