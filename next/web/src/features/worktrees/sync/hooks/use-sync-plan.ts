import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { dryRunQuery, planQuery, questionsQuery } from '@/api/queries/commands'
import type { DomainAnswered, SyncOnto, SyncRebase } from '@/api/types.gen'
import {
  moves,
  noFetch,
  pushOnly,
  rebasesOf,
  requestOf,
  riskOf,
  type Switches,
  unsafe,
  valuesOf,
} from '@/features/worktrees/sync/utils/plan'
import type { Tick } from '@/features/worktrees/sync/utils/wizard-state'
import { listed } from '@/features/worktrees/utils/listed'
import type { OutcomeRow } from '@/features/worktrees/utils/outcome'
import { refusalOf } from '@/features/worktrees/utils/sentence'

const reviewOf = (named: string[], picking: boolean): DomainAnswered[] => [
  picking ? { path: 'all', values: ['true'] } : { path: 'worktree', values: named },
  { path: 'force', values: ['true'] },
]

const cycleWords = 'its parents lead back to it'

// what the dialog plans: the review, forced over what step 1 shows so each dirty worktree shows its risk; the plan of
// the person's choices; and what holds the primary back
export function useSyncPlan({
  repo,
  asked,
  picking,
  checked,
  switches,
}: {
  repo: string
  asked: DomainAnswered[]
  picking: boolean
  checked: Record<string, Tick>
  switches: Switches
}) {
  // the form never fetches: it gives the header's own pick and each worktree's uncommitted changes
  const form = useQuery(questionsQuery(repo, 'sync'))
  const field = form.data?.fields.find((one) => one.path === 'worktree')
  const options = field?.kind === 'multi_select' ? field.options : []
  const picked = field?.kind === 'multi_select' ? field.picked : []
  const given = picking ? picked : (valuesOf(asked, 'worktree') ?? [])
  const changesOf = (worktree: string) => options.find((option) => option.value === worktree)?.changes

  // a cycle refuses the whole request: a selection reviews again without its members, which it lists as can't sync
  const first = useQuery(dryRunQuery(repo, 'sync', reviewOf(given, picking)))
  const cycle = !picking && given.length > 1 && first.data?.error?.code === 'sync.cycle' ? first.data.error : undefined
  const cycled = cycle?.params?.worktrees?.split(', ') ?? []
  const named = given.filter((name) => !cycled.includes(name))
  const second = useQuery({ ...dryRunQuery(repo, 'sync', reviewOf(named, false)), enabled: cycle !== undefined })
  const review = cycle ? second : first
  const reviewed = review.data?.error ? undefined : review.data?.plan
  const rows = (reviewed?.worktrees ?? []).filter((rebase) => rebase.standing !== 'in_cycle')
  const ontos = reviewed?.onto ?? []

  // a tick holds for the risk it was given under, an untick whatever the risk
  const ticked = (name: string, risk: string) => {
    const tick = checked[name]
    return tick && (!tick.on || tick.risk === risk) ? tick : undefined
  }
  const isNamed = (rebase: SyncRebase) =>
    ticked(rebase.worktree, riskOf(rebase))?.on ?? (named.includes(rebase.worktree) && !unsafe(rebase))
  const ontoRisk = (onto: SyncOnto) => (onto.blockers ?? []).join(',')
  const ontoTicked = (onto: SyncOnto) => ticked(onto.branch, ontoRisk(onto))?.on === true
  const going = rows.filter(isNamed)
  const untouched = going.length === named.length && going.every((rebase) => named.includes(rebase.worktree))
  const chosen = picking && untouched
  const forced = going.some(unsafe) || ontos.some((onto) => (onto.blockers ?? []).length > 0 && ontoTicked(onto))
  const answers = [...requestOf({ chosen, named: going.map((rebase) => rebase.worktree), forced, switches }), noFetch]
  const asking = reviewed !== undefined && (chosen || going.length > 0)
  const planned = useQuery({
    ...planQuery(repo, 'sync', { answers, readAt: review.dataUpdatedAt }),
    enabled: asking && !review.isFetching,
    placeholderData: keepPreviousData,
  })
  const plan = asking && !planned.isError && !planned.data?.error ? planned.data?.plan : undefined

  // forced for one dirty worktree checked, the plan forces every dirty one, main's fast-forward included: none moves
  // unless checked too
  const unconsented = [
    ...(plan?.onto ?? [])
      .filter((onto) => (onto.blockers ?? []).length > 0 && moves(onto) && !ontoTicked(onto))
      .map((onto) => onto.branch),
    ...(plan?.worktrees ?? [])
      .filter((rebase) => unsafe(rebase) && moves(rebase) && !isNamed(rebase))
      .map((rebase) => rebase.worktree),
  ]
  const fresh =
    plan !== undefined &&
    !planned.isPlaceholderData &&
    !planned.isFetching &&
    !review.isFetching &&
    unconsented.length === 0
  const refusal = review.data?.error ?? (asking ? planned.data?.error : undefined)
  const held = rows.filter((rebase) => named.includes(rebase.worktree) && unsafe(rebase) && !isNamed(rebase))
  const count = plan ? (pushOnly(plan) ? (plan.worktrees ?? []) : rebasesOf(plan)).filter(moves).length : 0

  const reparentOf = (worktree: string) => {
    const followUp = cycle
      ? cycle.follow_up
      : review.data?.warnings.find((warning) => warning.params?.worktree === worktree)?.follow_up
    return followUp?.command === 'reparent' && valuesOf(followUp.answers ?? [], 'worktree')?.includes(worktree)
      ? followUp
      : undefined
  }
  const cantSync: OutcomeRow[] = [
    ...cycled,
    ...(reviewed?.worktrees ?? []).filter((rebase) => rebase.standing === 'in_cycle').map((rebase) => rebase.worktree),
  ].map((worktree) => ({
    key: worktree,
    tone: 'failed',
    title: worktree,
    message: cycleWords,
    followUp: reparentOf(worktree),
  }))

  const why = () => {
    if (review.isFetching) return 'Fetching origin'
    if (refusal?.follow_up?.command === 'reparent')
      return `Reparent ${listed(valuesOf(refusal.follow_up.answers ?? [], 'worktree') ?? [])} to sync it`
    if (refusal) return refusalOf(refusal)
    if (planned.isFetching) return 'Planning'
    if (unconsented.length > 0) return `Check ${listed(unconsented)} too: forcing one forces every dirty worktree`
    if (going.length === 0 && !chosen) {
      const names = held.map((rebase) => rebase.worktree)
      if (names.length > 0) return `Check ${listed(names)} to sync ${names.length === 1 ? 'it' : 'them'} anyway`
      return 'Check a worktree to sync it'
    }
    return undefined
  }

  return {
    changesOf,
    review,
    reviewed: reviewed && { ...reviewed, worktrees: rows },
    rows,
    cantSync,
    isNamed,
    ontoTicked,
    ontoRisk,
    // a named worktree the review finds unsafe and the person has not checked: it needs their consent, on step 1
    consent: held.length > 0,
    asking,
    planned,
    plan,
    fingerprint: planned.data?.fingerprint,
    answers,
    fresh,
    refusal,
    empty: fresh && plan.empty === true,
    behind: ontos.filter(moves).map((onto) => onto.branch),
    count,
    why,
  }
}
