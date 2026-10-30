import type { Read } from '@/api/queries/commands'
import type { TreeDetail, TreePR, WorktreeCondition } from '@/api/types.gen'
import { overlaid } from '@/features/worktrees/utils/condition'

export type Node = {
  branch: string
  detail: TreeDetail
  pr: TreePR | null | undefined
  condition: WorktreeCondition
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
