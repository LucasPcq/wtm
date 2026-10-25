import type { Read } from '@/api/queries/commands'
import type { DomainFollowUp, TreeDetail, TreePR } from '@/api/types.gen'

// A Node is one branch of the tree as the screen draws it: the live read's detail, the pull request GitHub gave it
// the last time it was asked, what it needs next, and its children.
export type Node = {
  branch: string
  detail: TreeDetail
  pr: TreePR | null | undefined
  action: DomainFollowUp | undefined
  children: Node[]
}

// nodesOf is the forest of live, each node with what github read of it. The action is the core's: the live read's,
// but for a worktree GitHub says to clean, which the live read cannot know; a rebase to finish still comes first,
// as the core orders them.
export function nodesOf(live: Read<'tree'>, github: Read<'tree'> | undefined): Node[] {
  const asked = new Map(github?.items.map((item) => [item.subject, item.detail]))
  const nodes = new Map<string, Node>()
  for (const item of live.items) {
    if (!item.detail) continue
    const read = asked.get(item.subject)
    const [own] = item.detail.actions ?? []
    const clean = read?.actions?.find((action) => action.command === 'clean')
    nodes.set(item.subject, {
      branch: item.subject,
      detail: item.detail,
      pr: read?.pr,
      action: item.detail.rebasing ? own : (clean ?? own),
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
