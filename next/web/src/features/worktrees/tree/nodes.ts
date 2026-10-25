import type { Read } from '@/api/queries/commands'
import type { DomainFollowUp, TreeDetail, TreePR, WorktreeCondition } from '@/api/types.gen'
import { overlaid } from '../shared/condition'

// A Node is one branch of the tree as the screen draws it: the live read's detail, the pull request GitHub gave it
// the last time it was asked, its condition and what it needs next from both (overlaid), and its children.
export type Node = {
  branch: string
  detail: TreeDetail
  pr: TreePR | null | undefined
  condition: WorktreeCondition
  action: DomainFollowUp | undefined
  children: Node[]
}

// nodesOf is the forest of live, each node with what github read of it, its condition and action the core's, the two
// reads over each other as the web's one rule says (overlaid).
export function nodesOf(live: Read<'tree'>, github: Read<'tree'> | undefined): Node[] {
  const asked = new Map(github?.items.map((item) => [item.subject, item.detail]))
  const nodes = new Map<string, Node>()
  for (const item of live.items) {
    if (!item.detail) continue
    const read = asked.get(item.subject)
    const said = overlaid(item.detail, read)
    nodes.set(item.subject, {
      branch: item.subject,
      detail: item.detail,
      pr: read?.pr,
      condition: said.condition,
      action: said.actions?.[0],
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

// reparent is the follow-up that makes parent the parent of branch, and sync the one that then rebases it onto it:
// what dropping a worktree on another asks, until reparent runs here.
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
