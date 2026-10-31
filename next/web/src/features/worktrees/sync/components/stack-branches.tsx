import type { ReactNode } from 'react'
import type { StackNode } from '@/features/worktrees/sync/utils/stack'
import { cn } from '@/lib/utils'

// the rail runs under each box: an elbow to every child, hidden past the last one
export function StackBranches({
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
          {node.children.length > 0 && <StackBranches nodes={node.children} row={row} child />}
        </li>
      ))}
    </ul>
  )
}
