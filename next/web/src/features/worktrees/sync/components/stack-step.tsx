import type { SyncOnto, SyncPlan, SyncRebase } from '@/api/types.gen'
import { ListHeader } from '@/features/worktrees/components/list-header'
import { OptionSwitch } from '@/features/worktrees/sync/components/option-switch'
import { StackBranches } from '@/features/worktrees/sync/components/stack-branches'
import { type Box, StackRow } from '@/features/worktrees/sync/components/stack-row'
import { markedOf, moves, rootOf, type Switches, unsafe } from '@/features/worktrees/sync/utils/plan'
import { rebasesIn, type StackNode, searchedStack } from '@/features/worktrees/sync/utils/stack'
import { standingTone, standingWords, switchWords } from '@/features/worktrees/sync/utils/words'
import { changes, commits } from '@/features/worktrees/utils/counts'
import { listed } from '@/features/worktrees/utils/listed'
import { plural } from '@/lib/plural'

// step 1 holds every choice: the stack as a tree, each box one the core takes (a worktree named, a dirty one forced,
// main's fast-forward first), then the switches
export function StackStep({
  stack,
  plan,
  picking,
  isNamed,
  onCheck,
  ontoTicked,
  onOntoTick,
  changesOf,
  switches,
  onSwitch,
  query,
  onQuery,
}: {
  stack: StackNode[]
  plan: SyncPlan | undefined
  picking: boolean
  isNamed: (rebase: SyncRebase) => boolean
  onCheck: (rebases: SyncRebase[], on: boolean) => void
  ontoTicked: (onto: SyncOnto) => boolean
  onOntoTick: (onto: SyncOnto, on: boolean) => void
  changesOf: (worktree: string) => number | undefined
  switches: Switches
  onSwitch: (name: keyof Switches, on: boolean) => void
  query: string
  onQuery: (query: string) => void
}) {
  const all = rebasesIn(stack)
  const shown = searchedStack(stack, query)
  const planned = (worktree: string) => plan?.worktrees?.find((rebase) => rebase.worktree === worktree)
  const follows = (rebase: SyncRebase) => {
    const ran = planned(rebase.worktree)
    // a dirty one forced through another's consent keeps its own box, never "follows"
    return !isNamed(rebase) && !unsafe(rebase) && ran?.descendant === true && moves(ran)
  }
  const leftOut = (rebase: SyncRebase) => !isNamed(rebase) && (plan?.left_out ?? []).includes(rebase.worktree)
  const selectable = (rebase: SyncRebase) =>
    (rebase.standing === 'rebases' || rebase.standing === 'on_parent') && !follows(rebase) && !leftOut(rebase)

  const under = all.filter((rebase) => rebase.descendant)
  const marked = markedOf(plan, all)
  const following = (worktree: string) => marked.some((row) => row.worktree === worktree && row.descendant)
  const tops = all.filter((rebase) => !rebase.descendant).map((rebase) => rebase.worktree)
  const descendantsTitle =
    picking || tops.length > 1
      ? 'Sync the worktrees under the ones picked'
      : `Sync the ${plural(under.length, 'worktree')} under ${listed(tops)}`
  // a descendant checked anyway is named, yet follows all the same
  // a dirty one forced along, not checked, waits for its consent: it does not follow
  const followers = (plan?.worktrees ?? []).filter(
    (rebase) => following(rebase.worktree) && moves(rebase) && (!unsafe(rebase) || isNamed(rebase)),
  )
  const kept = (plan?.worktrees ?? []).filter((rebase) => rebase.descendant && !moves(rebase) && unsafe(rebase))
  const ontoMoving = stack.flatMap((node) => (node.kind === 'onto' && moves(node.onto) ? [node.name] : []))

  const descendantsLine = () => {
    if (!switches.descendants) {
      const left = plan?.left_out ?? []
      if (left.length === 0) return ''
      return `${listed(left)} ${left.length === 1 ? 'stays where it is, and needs' : 'stay where they are, and need'} a sync after.`
    }
    const follow = followers.map((rebase) => rebase.worktree)
    const parts = [
      follow.length > 0 && `${listed(follow)} ${follow.length === 1 ? 'follows' : 'follow'}`,
      kept.length > 0 &&
        `${listed(kept.map((rebase) => rebase.worktree))} ${kept.length === 1 ? 'is' : 'are'} kept (uncommitted changes)`,
    ].filter(Boolean)
    return parts.length > 0 ? `${parts.join('; ')}.` : ''
  }

  // a branch rebased onto: its box is the one ff_parents answer; a dirty one is fast-forwarded only checked itself, its
  // changes stashed around it
  const ontoRow = (onto: SyncOnto) => {
    const name = onto.branch
    const behind = onto.origin?.behind ?? 0
    const others = ontoMoving.filter((other) => other !== name)
    const along = others.length > 0 ? `, with ${listed(others)}` : ''
    const dirty = (onto.blockers ?? []).length > 0
    const forwarded = moves(plan?.onto?.find((one) => one.branch === name) ?? { phases: [] })
    const checked = switches.ff_parents && (!dirty || ontoTicked(onto))
    const box: Box | undefined = moves(onto)
      ? {
          checked,
          disabled: false,
          label: `Fast-forward ${name} first`,
          // checked, a dirty one takes its own consent too; unchecked, the parents are rebased onto as they stand
          onCheck: (on) => {
            if (dirty) onOntoTick(onto, on)
            onSwitch('ff_parents', on)
          },
        }
      : undefined
    const line = () => {
      if (!moves(onto)) return 'rebased onto as it stands'
      // forced through another's consent, a dirty one still waits for its own
      if (forwarded && !(dirty && !ontoTicked(onto)))
        return `fast-forwarded to origin first · ${commits(behind)}${along}`
      return `rebased onto as it stands · ${commits(behind)} behind origin${along}`
    }
    const attention = () => {
      if (!dirty) return []
      return [
        checked
          ? 'uncommitted changes: stashed, then put back'
          : 'uncommitted changes: check it to fast-forward it anyway',
      ]
    }
    return <StackRow name={name} box={box} attention={attention()} lines={[line()]} />
  }

  const rebaseRow = (rebase: SyncRebase) => {
    const name = rebase.worktree
    const label = `Sync ${name}`
    if (rebase.standing !== 'rebases' && rebase.standing !== 'on_parent') {
      const words = standingWords[rebase.standing](rebase)
      const amber = standingTone[rebase.standing] === 'attention'
      return (
        <StackRow
          name={name}
          box={{ checked: false, disabled: true, label }}
          attention={amber ? [words] : []}
          lines={amber ? [] : [words]}
        />
      )
    }
    if (follows(rebase)) {
      const root = rootOf(marked, name)
      return (
        <StackRow
          name={name}
          box={{
            checked: true,
            disabled: true,
            label,
            tooltip: `Comes with ${root}; turn off "${descendantsTitle}" to leave it`,
          }}
          lines={[`${standingWords.rebases(rebase)} · follows ${root}`]}
        />
      )
    }
    if (leftOut(rebase))
      return (
        <StackRow
          name={name}
          box={{ checked: false, disabled: true, label }}
          lines={[`stays on ${rebase.parent} as it is · needs a sync after`]}
        />
      )

    const named = isNamed(rebase)
    const dirty = changes(changesOf(name))
    const followsLine = named && following(name) ? ` · follows ${rootOf(marked, name)}` : ''
    const attention = () => {
      if (!unsafe(rebase)) return []
      return [named ? `${dirty}: stashed, then put back` : `${dirty}: check it to sync it anyway`]
    }
    return (
      <StackRow
        name={name}
        box={{ checked: named, disabled: false, label, onCheck: (on) => onCheck([rebase], on) }}
        attention={attention()}
        lines={[`${standingWords[rebase.standing](rebase)}${followsLine}`]}
      />
    )
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col rounded-lg border">
        <ListHeader
          total={all.length}
          visible={rebasesIn(shown).map((rebase) => rebase.worktree)}
          bulk={rebasesIn(shown)
            .filter((rebase) => selectable(rebase) && !unsafe(rebase))
            .map((rebase) => rebase.worktree)}
          checked={all.filter(isNamed).map((rebase) => rebase.worktree)}
          query={query}
          onQuery={onQuery}
          onCheck={(worktrees, on) =>
            onCheck(
              all.filter((rebase) => worktrees.includes(rebase.worktree) && selectable(rebase)),
              on,
            )
          }
        />
        <StackBranches
          nodes={shown}
          row={(node) => (node.kind === 'onto' ? ontoRow(node.onto) : rebaseRow(node.rebase))}
        />
      </div>

      <div className="flex flex-col divide-y rounded-lg border">
        {(picking || under.length > 0) && (
          <OptionSwitch
            title={descendantsTitle}
            line={descendantsLine()}
            on={switches.descendants}
            onSwitch={(on) => onSwitch('descendants', on)}
          />
        )}
        {(['push', 'keep_conflict'] as const).map((name) => (
          <OptionSwitch
            key={name}
            title={switchWords[name].title}
            line={switches[name] ? switchWords[name].on : switchWords[name].off}
            on={switches[name]}
            onSwitch={(on) => onSwitch(name, on)}
          />
        ))}
      </div>
    </div>
  )
}
