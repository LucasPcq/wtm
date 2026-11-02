import type { ReactNode } from 'react'
import { Rail } from '@/features/worktrees/components/rail'
import type { StackNode } from '@/features/worktrees/sync/utils/stack'

export function StackBranches({ nodes, row }: { nodes: StackNode[]; row: (node: StackNode) => ReactNode }) {
  const items = (level: StackNode[]) =>
    level.map((node) => (
      <li key={node.name} aria-label={node.name}>
        {row(node)}
        {node.children.length > 0 && (
          // the rail runs under the box's centre
          <Rail elbow={17} indent={12} className="ml-[7px]">
            {items(node.children)}
          </Rail>
        )}
      </li>
    ))

  return (
    <ul aria-label="Worktrees" className="flex flex-col px-4 py-2">
      {items(nodes)}
    </ul>
  )
}
