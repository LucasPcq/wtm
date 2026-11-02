import { Rail } from '@/features/worktrees/components/rail'
import { NodeRow } from '@/features/worktrees/tree/components/node-row'
import type { Node } from '@/features/worktrees/tree/utils/nodes'
import type { Moving, Target } from '@/features/worktrees/tree/utils/targets'
import type { Subject } from '@/features/worktrees/utils/actions'

export type View = {
  repo: string
  subjects: Subject[]
  query: string
  open: string | undefined
  underWay: Record<string, string>
  movable: string[]
  // undefined while nothing is dragged
  target: ((node: Node) => Target) | undefined
  prefetch: (branch: string) => void
  moving: Moving | undefined
  landed: () => void
}

export function Branch({ node, view }: { node: Node; view: View }) {
  return (
    <li>
      <NodeRow node={node} view={view} />
      {node.children.length > 0 && (
        // the rail runs under the parent's dot: the row's padding, its grip, the gap, half the dot
        <Rail elbow={24} indent={20} className="ml-[39px]">
          {node.children.map((next) => (
            <Branch key={next.branch} node={next} view={view} />
          ))}
        </Rail>
      )}
    </li>
  )
}
