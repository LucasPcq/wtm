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
  hidden: number
}

const shownOf = (job: RunpsJob): ShownState => {
  if (job.state === 'crashed') return 'crashed'
  return hasEnded(job) ? 'ended' : 'running'
}

const chipKeeps: Record<Chip, (job: RunpsJob) => boolean> = {
  all: () => true,
  running: (job) => job.state === 'running',
  crashed: (job) => job.state === 'crashed',
}

// what the page lists of groups under filter: pinned jobs on top and out of their group; a worktree's ended jobs on
// its quiet row unless the filter lists them; how many jobs the filter hides, which the toolbar says
export function visible({
  groups,
  filter,
  search,
  here,
  pinned,
  quiet,
}: {
  groups: RunningGroup[]
  filter: RunFilter
  search: string
  here: string | undefined
  pinned: (id: string) => boolean
  quiet: (key: string) => boolean
}): Visible {
  const query = search.trim().toLowerCase()
  const inWorktree = (group: RunningGroup) => {
    if (filter.worktree === undefined) return true
    if (filter.worktree === 'shared') return group.shared
    if (filter.worktree === 'here') return !group.shared && group.path === here
    return !group.shared && group.worktree === filter.worktree
  }
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
    const ended = calmly ? group.ended : group.ended.filter((job) => keeps(group, job))
    const endedShown = !calmly || quiet(group.key) ? ended : []
    shown += endedShown.length
    if (calmly && !quiet(group.key)) quietly += ended.length
    // a worktree whose every job ended leaves the page: its quiet row would stand alone
    if (rows.length === 0 && endedShown.length === 0) return []
    return [{ ...group, rows, ended: calmly ? ended : [], endedShown }]
  })

  return { pinned: pins, groups: listed, counts, hidden: Math.max(all.length - shown - quietly, 0) }
}
