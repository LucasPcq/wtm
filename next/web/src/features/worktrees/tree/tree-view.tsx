import { DragDropProvider, KeyboardSensor, PointerSensor } from '@dnd-kit/react'
import { useState } from 'react'
import { useRead } from '@/features/worktrees/hooks/use-read'
import type { WorktreesSearch } from '@/features/worktrees/search'
import { Loading, ReadFailed } from '@/features/worktrees/shared/loading'
import { Branch, type Drop, type View } from '@/features/worktrees/tree/branch'
import { nodesOf } from '@/features/worktrees/tree/nodes'

export function TreeView({
  repo,
  search,
  underWay,
}: {
  repo: string
  search: WorktreesSearch
  underWay: Record<string, string>
}) {
  const { live, github } = useRead(repo, 'tree')
  const [dropped, setDropped] = useState<Drop>()

  if (live.isPending) return <Loading />
  if (live.isError) return <ReadFailed message={live.error.message} />

  const view: View = {
    query: search.q ?? '',
    open: search.worktree,
    underWay,
    dropped,
    dismiss: () => setDropped(undefined),
  }

  return (
    <DragDropProvider
      sensors={[PointerSensor, KeyboardSensor]}
      onDragEnd={({ canceled, operation }) => {
        const branch = operation.source?.id
        const parent = operation.target?.id
        if (canceled || typeof branch !== 'string' || typeof parent !== 'string' || branch === parent) return
        setDropped({ branch, parent })
      }}
    >
      <ul className="flex flex-col gap-4 px-5 py-4" aria-label="Worktrees as a tree">
        {nodesOf(live.data, github.data).map((root) => (
          <Branch key={root.branch} node={root} view={view} />
        ))}
      </ul>
    </DragDropProvider>
  )
}
