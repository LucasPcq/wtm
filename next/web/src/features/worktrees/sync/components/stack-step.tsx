import { type ReactNode, useId } from 'react'
import type { SyncPlan, SyncRebase } from '@/api/types.gen'
import { Switch } from '@/components/ui/switch'
import { ListHeader } from '@/features/worktrees/components/list-header'
import { type Box, StackRow } from '@/features/worktrees/sync/components/stack-row'
import { markedOf, moves, rootOf, type Switches, unsafe } from '@/features/worktrees/sync/utils/plan'
import { rebasesIn, type StackNode, searchedStack } from '@/features/worktrees/sync/utils/stack'
import {
  changes,
  commits,
  names,
  standingTone,
  standingWords,
  switchWords,
} from '@/features/worktrees/sync/utils/words'
import { plural } from '@/lib/plural'
import { cn } from '@/lib/utils'

// step 1 holds every choice: the stack as a tree, each box one the core takes (a worktree named, a dirty one forced,
// main's fast-forward first), then the switches
export function StackStep({
  stack,
  plan,
  picking,
  isNamed,
  onCheck,
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
    return !isNamed(rebase) && ran?.descendant === true && moves(ran)
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
      : `Sync the ${plural(under.length, 'worktree')} under ${names(tops)}`
  // a descendant checked anyway is named, yet follows all the same
  const followers = (plan?.worktrees ?? []).filter((rebase) => following(rebase.worktree) && moves(rebase))
  const kept = (plan?.worktrees ?? []).filter((rebase) => rebase.descendant && !moves(rebase) && unsafe(rebase))
  const ontoMoving = stack.flatMap((node) => (node.kind === 'onto' && moves(node.onto) ? [node.name] : []))

  const descendantsLine = () => {
    if (!switches.descendants) {
      const left = plan?.left_out ?? []
      if (left.length === 0) return ''
      return `${names(left)} ${left.length === 1 ? 'stays where it is, and needs' : 'stay where they are, and need'} a sync after.`
    }
    const follow = followers.map((rebase) => rebase.worktree)
    const parts = [
      follow.length > 0 && `${names(follow)} ${follow.length === 1 ? 'follows' : 'follow'}`,
      kept.length > 0 &&
        `${names(kept.map((rebase) => rebase.worktree))} ${kept.length === 1 ? 'is' : 'are'} kept (uncommitted changes)`,
    ].filter(Boolean)
    return parts.length > 0 ? `${parts.join('; ')}.` : ''
  }

  const ontoRow = (name: string, behind: number, canMove: boolean) => {
    const others = ontoMoving.filter((other) => other !== name)
    const along = others.length > 0 ? `, with ${names(others)}` : ''
    const box: Box | undefined = canMove
      ? {
          checked: switches.ff_parents,
          disabled: false,
          label: `Fast-forward ${name} first`,
          onCheck: (on) => onSwitch('ff_parents', on),
        }
      : undefined
    const dirty = plan?.onto?.find((onto) => onto.branch === name)?.blockers?.length
    const line = () => {
      if (!canMove) return 'rebased onto as it stands'
      if (switches.ff_parents) return `fast-forwarded to origin first · ${commits(behind)}${along}`
      return `rebased onto as it stands · ${commits(behind)} behind origin${along}`
    }
    return (
      <StackRow
        name={name}
        box={box}
        attention={dirty ? ['uncommitted changes: kept as it is'] : []}
        lines={[line()]}
      />
    )
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
        <Branches
          nodes={shown}
          row={(node) =>
            node.kind === 'onto'
              ? ontoRow(node.name, node.onto.origin?.behind ?? 0, moves(node.onto))
              : rebaseRow(node.rebase)
          }
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

// the rail runs under each box: an elbow to every child, hidden past the last one
function Branches({
  nodes,
  row,
  child = false,
}: {
  nodes: StackNode[]
  row: (node: StackNode) => ReactNode
  child?: boolean
}) {
  return (
    <ul
      aria-label={child ? undefined : 'Worktrees'}
      className={cn('flex flex-col', child ? 'ml-[7px] border-l pl-[12px]' : 'px-4 py-2')}
    >
      {nodes.map((node) => (
        <li
          key={node.name}
          aria-label={node.name}
          className={cn(
            'relative',
            child && 'before:absolute before:top-[17px] before:-left-[12px] before:w-[10px] before:border-t',
            child &&
              'last:after:absolute last:after:top-[18px] last:after:bottom-0 last:after:-left-[13px] last:after:w-px last:after:bg-background',
          )}
        >
          {row(node)}
          {node.children.length > 0 && <Branches nodes={node.children} row={row} child />}
        </li>
      ))}
    </ul>
  )
}

function OptionSwitch({
  title,
  line,
  on,
  onSwitch,
}: {
  title: string
  line: string
  on: boolean
  onSwitch: (on: boolean) => void
}) {
  const id = useId()

  return (
    <div className="flex items-start gap-4 px-4 py-3">
      <Switch id={id} checked={on} onCheckedChange={onSwitch} className="mt-0.5" />
      <span className="flex min-w-0 flex-col gap-1">
        <label htmlFor={id} className="cursor-pointer text-sm font-medium">
          {title}
        </label>
        {line && <span className="text-xs text-muted-foreground">{line}</span>}
      </span>
    </div>
  )
}
