import { NodeRow } from '@/features/worktrees/tree/components/node-row'
import type { Node } from '@/features/worktrees/tree/utils/nodes'
import { cn } from '@/lib/utils'

export type Drop = { branch: string; parent: string }

export type View = {
  query: string
  open: string | undefined
  underWay: Record<string, string>
  dropped: Drop | undefined
  dismiss: () => void
}

export function Branch({ node, view, child = false }: { node: Node; view: View; child?: boolean }) {
  return (
    <li
      className={cn(
        'relative',
        // an elbow from the rail; past the last child, the rail is hidden
        child && 'before:absolute before:top-6 before:-left-5 before:w-5 before:border-t',
        child &&
          'last:after:absolute last:after:top-6 last:after:bottom-0 last:after:-left-[21px] last:after:w-px last:after:bg-background',
      )}
    >
      <NodeRow node={node} view={view} />
      {node.children.length > 0 && (
        // the rail runs under the parent's dot: the row's padding, its grip, the gap, half the dot
        <ul className="ml-[39px] flex flex-col border-l pl-5">
          {node.children.map((next) => (
            <Branch key={next.branch} node={next} view={view} child />
          ))}
        </ul>
      )}
    </li>
  )
}
