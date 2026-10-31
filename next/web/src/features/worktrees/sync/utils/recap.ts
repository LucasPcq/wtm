import type { DomainAnswered, DomainFollowUp, DomainWarning, SyncPlan, SyncRebase } from '@/api/types.gen'
import type { Left } from '@/features/worktrees/components/left-item'
import { moves, rebasesOf, unsafe } from '@/features/worktrees/sync/utils/plan'
import {
  changes,
  commits,
  failureHelp,
  failureWords,
  isFailure,
  names,
  standingTone,
  standingWords,
} from '@/features/worktrees/sync/utils/words'
import { subjectsOf } from '@/features/worktrees/utils/actions'
import { type OutcomeGroup, type OutcomeRow, outputOf } from '@/features/worktrees/utils/outcome'
import type { Progress } from '@/features/worktrees/utils/progress'
import { sentence } from '@/features/worktrees/utils/sentence'

// the fixes a sync warning offers, each a sync the dialog plans again
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
  left,
  changesOf,
  onReparent,
}: {
  plan: SyncPlan
  review: SyncRebase[]
  warnings: DomainWarning[]
  left: Left[]
  changesOf: (worktree: string) => number | undefined
  onReparent: (followUp: DomainFollowUp) => void
}): OutcomeRow[] {
  const kept = (plan.worktrees ?? []).filter((rebase) => !moves(rebase))
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
    ...left.map(({ worktree, why }) => ({ key: worktree, tone: 'muted' as const, title: worktree, message: why })),
  ]
}

// what a run did, in the house's groups: what couldn't sync (with what stopped under it), what to check, the
// worktrees synced, then the branches fast-forwarded first; a follow-up the web runs is a button on the row it serves
export function recapOf(
  progress: Progress<'sync'>,
  run: (answers: DomainAnswered[]) => void,
): { groups: OutcomeGroup[]; warnings: DomainWarning[]; synced: number; asked: number; pushed: number } {
  const envelope = progress.envelope
  const plan = envelope?.plan
  const items = envelope?.items ?? []
  const warnings = envelope?.warnings ?? []
  const followUps = envelope?.follow_ups ?? []
  const onto = (plan?.onto ?? []).map((branch) => branch.branch)
  const planned = (worktree: string) => plan?.worktrees?.find((rebase) => rebase.worktree === worktree)
  const followUpOf = (code: string, subject: string) =>
    followUps.find((followUp) => followUp.code === code && subjectsOf(followUp).includes(subject))

  const failedRow = (item: (typeof items)[number]): OutcomeRow => {
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
      tone: 'failed',
      title: item.subject,
      message: error && (isFailure(error.code) ? failureWords[error.code](params) : sentence(error.message)),
      note: error && isFailure(error.code) ? failureHelp[error.code](params) : undefined,
      output: output(),
      go: followUpOf('sync.go', item.subject),
      act: again && (() => run(again.answers ?? [])),
      actLabel: resolve ? 'Run again, keep the conflict' : 'Sync the rest',
    }
  }

  const stoppedRow = (item: (typeof items)[number]): OutcomeRow => {
    const parent = planned(item.subject)?.parent
    return {
      key: item.subject,
      tone: 'muted',
      title: item.subject,
      message: item.reason === 'blocked' ? `blocked: ${parent} didn't sync` : 'not started: cancelled first',
    }
  }

  const push = followUps.find((followUp) => followUp.code === 'sync.push')
  const unpushed = push ? subjectsOf(push) : []
  const leftOut = plan?.left_out ?? []
  // a warning whose fix is a sync is run here, never copied: sync runs in the web
  const syncing = warnings.filter((warning) => warning.follow_up?.command === 'sync')
  const check: OutcomeRow[] = [
    ...syncing.map((warning) => {
      const answers = warning.follow_up?.answers ?? []
      return {
        key: `${warning.code}-${warning.message}`,
        tone: 'attention' as const,
        message: sentence(warning.message),
        act: () => run(answers),
        actLabel: followUpWords[warning.follow_up?.code ?? ''] ?? 'Sync',
      }
    }),
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
    ...(leftOut.length > 0
      ? [
          {
            key: 'left-out',
            tone: 'attention' as const,
            message: `${names(leftOut)} ${leftOut.length === 1 ? 'needs' : 'need'} a sync`,
            act: () => run([{ path: 'worktree', values: leftOut }]),
            actLabel: leftOut.length === 2 ? 'Sync both' : 'Sync them',
          },
        ]
      : []),
  ]

  const done = items.filter((item) => item.status === 'done' && item.reason !== 'unchanged')
  // a push origin refused fails its item, yet the rebase before it stands
  const rebased = items.filter(
    (item) => item.status === 'done' || (item.status === 'failed' && item.error?.code === 'sync.push_failed'),
  )
  const synced = rebased.filter(
    (item) => !onto.includes(item.subject) && (planned(item.subject)?.phases ?? []).includes('rebase'),
  )

  return {
    groups: [
      {
        kind: 'failed',
        rows: items.flatMap((item) => {
          if (item.status === 'failed') return [failedRow(item)]
          if (item.status === 'cancelled' || item.reason === 'blocked' || item.reason === 'interrupted')
            return [stoppedRow(item)]
          return []
        }),
      },
      { kind: 'check', rows: check },
      {
        kind: 'done',
        rows: done
          .filter((item) => !onto.includes(item.subject))
          .map((item) => {
            const detail = item.detail
            const rebased = (planned(item.subject)?.phases ?? []).includes('rebase')
            const where = rebased && detail ? `onto ${detail.parent} · ${commits(detail.commits)}` : ''
            return {
              key: item.subject,
              tone: 'ok' as const,
              title: item.subject,
              message: [where, detail?.pushed && 'pushed'].filter(Boolean).join(' · '),
              path: detail?.path,
            }
          }),
      },
      {
        kind: 'along',
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
    synced: synced.length,
    asked: plan ? rebasesOf(plan).length : 0,
    pushed: done.filter((item) => item.detail?.pushed).length,
  }
}
