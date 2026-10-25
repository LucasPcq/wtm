import type { Read } from '@/api/queries/commands'
import type { DomainFollowUp, TreeDetail, TreePR, WorktreeCondition } from '@/api/types.gen'
import { overlaid } from '@/features/worktrees/utils/condition'

export type Node = {
  branch: string
  detail: TreeDetail
  pr: TreePR | null | undefined
  condition: WorktreeCondition
  action: DomainFollowUp | undefined
  children: Node[]
}

export function nodesOf(live: Read<'tree'>, github: Read<'tree'> | undefined): Node[] {
  const asked = new Map(github?.items.map((item) => [item.subject, item.detail]))
  const nodes = new Map<string, Node>()
  for (const item of live.items) {
    if (!item.detail) continue
    const read = asked.get(item.subject)
    const shown = overlaid(item.detail, read)
    nodes.set(item.subject, {
      branch: item.subject,
      detail: item.detail,
      pr: read?.pr,
      condition: shown.condition,
      action: shown.actions?.[0],
      children: [],
    })
  }

  const roots: Node[] = []
  for (const node of nodes.values()) {
    const parent = node.detail.parent === null ? undefined : nodes.get(node.detail.parent)
    if (parent) parent.children.push(node)
    else roots.push(node)
  }
  return roots
}

// What dropping a worktree on another asks, until reparent runs here: record the new parent, then rebase onto it.
export const reparent = (branch: string, parent: string): DomainFollowUp => ({
  command: 'reparent',
  answers: [
    { path: 'worktree', values: [branch] },
    { path: 'to', values: [parent] },
  ],
  code: 'web.reparent',
  message: `make ${parent} the parent of ${branch}`,
})

export const sync = (branch: string): DomainFollowUp => ({
  command: 'sync',
  answers: [{ path: 'worktree', values: [branch] }],
  code: 'web.sync',
  message: `rebase ${branch} onto its new parent`,
})
