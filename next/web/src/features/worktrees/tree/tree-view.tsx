import { DragDropProvider, KeyboardSensor, PointerSensor } from '@dnd-kit/react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { questionsQuery } from '@/api/queries/commands'
import { Loading } from '@/features/worktrees/components/loading'
import { ReadFailed } from '@/features/worktrees/components/read-failed'
import { useRead } from '@/features/worktrees/hooks/use-read'
import { useReparentNow } from '@/features/worktrees/hooks/use-reparent-now'
import { Branch, type View } from '@/features/worktrees/tree/components/branch'
import { nodesOf } from '@/features/worktrees/tree/utils/nodes'
import { dropOn, targetOf } from '@/features/worktrees/tree/utils/targets'
import type { Subject } from '@/features/worktrees/utils/actions'
import { requestOf } from '@/features/worktrees/utils/reparent'
import type { WorktreesSearch } from '@/features/worktrees/utils/search'

const parentsOf = (repo: string, worktree: string) =>
  questionsQuery(repo, 'reparent', [{ path: 'worktree', values: [worktree] }])

export function TreeView({
  repo,
  search,
  underWay,
  subjects,
}: {
  repo: string
  search: WorktreesSearch
  underWay: Record<string, string>
  subjects: Subject[]
}) {
  const client = useQueryClient()
  const { live, github } = useRead(repo, 'tree')
  const reparentNow = useReparentNow(repo)
  const [dragging, setDragging] = useState<string>()
  // the grip a keyboard drop came from: the row is drawn again under its new parent, and focus goes back to it
  const [refocus, setRefocus] = useState<string>()
  // which nodes move, and where each may land, are reparent's questions: the Tree decides none of it
  const movable = useQuery(questionsQuery(repo, 'reparent'))
  const parents = useQuery({ ...parentsOf(repo, dragging ?? ''), enabled: dragging !== undefined })

  if (live.isPending) return <Loading />
  if (live.isError) return <ReadFailed message={live.error.message} />

  const worktreeField = movable.data?.fields.find((field) => field.path === 'worktree')
  const options = worktreeField?.kind === 'multi_select' ? worktreeField.options : []
  const parent = options.find((option) => option.value === dragging)?.parent
  const target = (branch: string, condition: Parameters<typeof targetOf>[0]['condition']) =>
    targetOf({ branch, form: parents.data, parent, condition })

  const view: View = {
    repo,
    subjects,
    query: search.q ?? '',
    open: search.worktree,
    underWay,
    movable: options.map((option) => option.value),
    target: dragging === undefined ? undefined : (node) => target(node.branch, node.condition),
    // asked again only past a while: an event asks it again anyway
    prefetch: (branch) => void client.prefetchQuery({ ...parentsOf(repo, branch), staleTime: 30_000 }),
    refocus,
    focused: () => setRefocus(undefined),
  }
  const nodes = nodesOf(live.data, github.data)
  const conditionOf = (branch: string) =>
    live.data.items.find((item) => item.subject === branch)?.detail?.condition ?? 'no_worktree'

  return (
    <DragDropProvider
      sensors={[PointerSensor, KeyboardSensor]}
      onDragStart={({ operation }) => {
        const branch = operation.source?.id
        if (typeof branch === 'string') setDragging(branch)
      }}
      onDragEnd={({ canceled, operation }) => {
        setDragging(undefined)
        const branch = operation.source?.id
        if (typeof branch === 'string') setRefocus(branch)
        const to = operation.target?.id
        if (canceled || typeof branch !== 'string' || typeof to !== 'string' || branch === to) return
        const drop = dropOn(target(to, conditionOf(to)))
        // said aloud too: a screen reader hears no tooltip
        if (typeof drop === 'object') return void toast.info(drop.refused)
        if (drop === 'none') return
        void reparentNow([
          {
            command: 'reparent',
            answers: requestOf({ worktrees: [branch], to }),
            code: 'web.reparent',
            message: `move ${branch} onto ${to}`,
          },
        ])
      }}
    >
      <ul className="flex flex-col gap-4 px-5 py-4" aria-label="Worktrees as a tree">
        {nodes.map((root) => (
          <Branch key={root.branch} node={root} view={view} />
        ))}
      </ul>
    </DragDropProvider>
  )
}
