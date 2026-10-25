// The columns a person may show besides Branch, which is always there, in the order the table and the panel list them.
export const columnIds = [
  'state',
  'behind',
  'origin',
  'changes',
  'pr',
  'checks',
  'running',
  'address',
  'active',
  'parent',
  'created',
  'path',
] as const
export type ColumnId = (typeof columnIds)[number]

export const columnText: Record<ColumnId, { label: string; hint: string }> = {
  state: { label: 'State', hint: 'clean, changed, rebasing, missing' },
  behind: { label: 'Behind', hint: 'commits its parent has that it lacks' },
  origin: { label: 'Origin', hint: 'ahead and behind its remote branch' },
  changes: { label: 'Changes', hint: 'changed and untracked files' },
  pr: { label: 'Pull request', hint: 'number and state, as last read' },
  checks: { label: 'Checks', hint: 'CI of an open pull request' },
  running: { label: 'Running', hint: 'jobs up in this worktree' },
  address: { label: 'Address', hint: 'URL of the jobs up' },
  active: { label: 'Active', hint: 'when its branch last moved' },
  parent: { label: 'Parent', hint: 'the branch it was created from' },
  created: { label: 'Created', hint: 'when wtm created it' },
  path: { label: 'Path', hint: 'its folder on disk' },
}

export const columnGroups: { title: string; columns: ColumnId[] }[] = [
  { title: 'Git', columns: ['state', 'behind', 'origin', 'changes'] },
  { title: 'Review', columns: ['pr', 'checks'] },
  { title: 'Run', columns: ['running', 'address'] },
  { title: 'Details', columns: ['active', 'parent', 'created', 'path'] },
]

export const defaultColumns: ColumnId[] = ['state', 'running', 'active']

// shownColumns is a stored choice as the columns to show, in the table's order: what it names that is a column,
// the default when nothing was stored or it does not read.
export function shownColumns(stored: unknown): ColumnId[] {
  if (!Array.isArray(stored)) return defaultColumns
  return columnIds.filter((id) => stored.includes(id))
}

// toggled is shown with column added, or removed when it is shown.
export function toggled(shown: ColumnId[], column: ColumnId): ColumnId[] {
  return shown.includes(column)
    ? shown.filter((id) => id !== column)
    : columnIds.filter((id) => id === column || shown.includes(id))
}
