import type { RunpsJob } from '@/api/types.gen'
import { hasEnded, type RunningGroup, type RunningRow } from '@/features/worktrees/running/utils/groups'
import type { Chip, RunFilter, ShownState } from '@/features/worktrees/store/ui-store'

// a job pinned to the top: its worktree and its name
export const pinOf = (worktree: string, job: string) => `${worktree}/${job}`

export type Pinned = { group: RunningGroup; row: RunningRow & { job: RunpsJob } }

export type Visible = {
  pinned: Pinned[]
  // each group's rows as filtered; `ended` are its ended jobs the quiet row hides, `endedShown` those it shows
  groups: (RunningGroup & { endedShown: RunpsJob[] })[]
  counts: Record<Chip, number>
  states: Record<ShownState, number>
  hidden: number
}

const shownOf = (job: RunpsJob): ShownState => {
  if (job.state === 'crashed') return 'crashed'
  return hasEnded(job) ? 'ended' : 'running'
}

// a chip and the Filter menu's states say running alike: up or starting
const chipKeeps: Record<Chip, (job: RunpsJob) => boolean> = {
  all: () => true,
  running: (job) => shownOf(job) === 'running',
  crashed: (job) => shownOf(job) === 'crashed',
}

// what the page lists of groups under filter: pinned jobs on top and out of their group; a worktree's ended jobs on
// its quiet row unless the filter lists them; how many jobs the filter hides, which the toolbar says
export function visible({
  groups,
  filter,
  search,
  pinned,
  quiet,
}: {
  groups: RunningGroup[]
  filter: RunFilter
  search: string
  pinned: (id: string) => boolean
  quiet: (key: string) => boolean
}): Visible {
  const query = search.trim().toLowerCase()
  const inWorktree = (group: RunningGroup) => filter.worktree === undefined || group.key === filter.worktree
  const keeps = (group: RunningGroup, job: RunpsJob) =>
    inWorktree(group) &&
    chipKeeps[filter.chip](job) &&
    (filter.chip !== 'all' || filter.states.includes(shownOf(job))) &&
    (query === '' || job.name.toLowerCase().includes(query) || group.worktree.toLowerCase().includes(query))

  const all = groups.flatMap((group) => [
    ...group.rows.flatMap((row) => ('job' in row ? [row.job] : [])),
    ...group.ended,
  ])
  const counts: Record<Chip, number> = {
    all: all.length,
    running: all.filter(chipKeeps.running).length,
    crashed: all.filter(chipKeeps.crashed).length,
  }
  const states: Record<ShownState, number> = {
    running: counts.running,
    crashed: counts.crashed,
    ended: all.filter((job) => shownOf(job) === 'ended').length,
  }

  const pins: Pinned[] = []
  let shown = 0
  let quietly = 0
  const listed = groups.flatMap((group) => {
    const rows = group.rows.filter((row) => {
      if (!('job' in row)) return inWorktree(group)
      if (!keeps(group, row.job)) return false
      shown += 1
      if (!pinned(pinOf(group.worktree, row.name))) return true
      pins.push({ group, row })
      return false
    })
    // a worktree's ended jobs show on its quiet row, unless the filter lists them among the others
    const calmly = filter.chip === 'all' && !filter.states.includes('ended') && query === '' && inWorktree(group)
    // a pinned job that ended stays on top, quiet, as a pinned job that runs does
    const endedPinned = group.ended.filter(
      (job) => pinned(pinOf(group.worktree, job.name)) && (calmly || keeps(group, job)),
    )
    for (const job of endedPinned) pins.push({ group, row: { name: job.name, job } })
    shown += endedPinned.length
    const unpinned = group.ended.filter((job) => !endedPinned.includes(job))
    const ended = calmly ? unpinned : unpinned.filter((job) => keeps(group, job))
    const endedShown = !calmly || quiet(group.key) ? ended : []
    shown += endedShown.length
    if (calmly && !quiet(group.key)) quietly += ended.length
    // a worktree whose every job ended leaves the page: its quiet row would stand alone
    if (rows.length === 0 && endedShown.length === 0) return []
    return [{ ...group, rows, ended: calmly ? ended : [], endedShown }]
  })

  return { pinned: pins, groups: listed, counts, states, hidden: Math.max(all.length - shown - quietly, 0) }
}
