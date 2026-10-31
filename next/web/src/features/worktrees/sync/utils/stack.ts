import type { SyncOnto, SyncPlan, SyncRebase } from '@/api/types.gen'

export type StackNode =
  | { kind: 'onto'; name: string; onto: SyncOnto; children: StackNode[] }
  | { kind: 'rebase'; name: string; rebase: SyncRebase; children: StackNode[] }

// the review's branches as the stacks they make: each branch rebased onto first is a root, each worktree under its
// parent; a worktree whose parent is in neither (gone, unrecorded), or whose parents lead back to it, is a root of its own
export function stackOf(plan: Pick<SyncPlan, 'onto' | 'worktrees'>): StackNode[] {
  const nodes: StackNode[] = [
    ...(plan.onto ?? []).map((onto): StackNode => ({ kind: 'onto', name: onto.branch, onto, children: [] })),
    ...(plan.worktrees ?? []).map(
      (rebase): StackNode => ({ kind: 'rebase', name: rebase.worktree, rebase, children: [] }),
    ),
  ]
  const byName = new Map(nodes.map((node) => [node.name, node]))

  return nodes.filter((node) => {
    if (node.kind === 'onto' || node.rebase.standing === 'in_cycle') return true
    const parent = byName.get(node.rebase.parent)
    if (!parent) return true
    parent.children.push(node)
    return false
  })
}

// a search keeps a branch that matches, with the branches above it, so the stack still reads
export function searchedStack(nodes: StackNode[], query: string): StackNode[] {
  const needle = query.trim().toLowerCase()
  if (!needle) return nodes
  return nodes.flatMap((node) => {
    const children = searchedStack(node.children, query)
    return node.name.toLowerCase().includes(needle) || children.length > 0 ? [{ ...node, children }] : []
  })
}

export const rebasesIn = (nodes: StackNode[]): SyncRebase[] =>
  nodes.flatMap((node) => [...(node.kind === 'rebase' ? [node.rebase] : []), ...rebasesIn(node.children)])
