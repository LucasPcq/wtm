import { DragDropProvider, KeyboardSensor, PointerSensor, useDraggable, useDroppable } from '@dnd-kit/react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { CircleCheck, CircleDashed, CircleDot, CircleSlash, GitMerge, GripVertical, TriangleAlert } from 'lucide-react'
import { useState } from 'react'
import { githubQuery, readQuery } from '@/api/queries/commands'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverAnchor,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { cn } from '@/lib/utils'
import { Loading, ReadFailed } from '../shared/loading'
import { UnderWay } from '../shared/under-way'
import type { WorktreesSearch } from '../search'
import { CommandLine } from '../shared/command-line'
import { Glyph } from '../shared/glyph'
import { reparent, sync, nodesOf, type Node } from './nodes'
import { sentence } from '../shared/sentence'

type Drop = { branch: string; parent: string }

// TreeView is each worktree under the branch it was created from, what each needs next, and dragging one onto
// another to change its parent; the search dims the branches it does not find.
export function TreeView(props: { repo: string; search: WorktreesSearch; underWay: Record<string, string> }) {
  const live = useQuery(readQuery(props.repo, 'tree'))
  const github = useQuery(githubQuery(props.repo, 'tree'))
  const [dropped, setDropped] = useState<Drop>()
  if (live.isPending) return <Loading />
  if (live.isError) return <ReadFailed message={live.error.message} />

  const view: View = {
    query: (props.search.q ?? '').trim().toLowerCase(),
    open: props.search.worktree,
    underWay: props.underWay,
    dropped,
    dismiss: () => setDropped(undefined),
  }
  return (
    <DragDropProvider
      sensors={[PointerSensor, KeyboardSensor]}
      onDragEnd={(event) => {
        const branch = event.operation.source?.id
        const parent = event.operation.target?.id
        if (event.canceled || typeof branch !== 'string' || typeof parent !== 'string' || branch === parent) return
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

type View = {
  query: string
  open: string | undefined
  underWay: Record<string, string>
  dropped: Drop | undefined
  dismiss: () => void
}

function Branch({ node, view, child }: { node: Node; view: View; child?: boolean }) {
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

function NodeRow({ node, view }: { node: Node; view: View }) {
  const detail = node.detail
  const movable = !detail.main && !detail.virtual
  const { ref: dragRef, handleRef, isDragSource } = useDraggable({ id: node.branch, disabled: !movable })
  const { ref: dropRef, isDropTarget } = useDroppable({ id: node.branch })
  const found = view.query === '' || node.branch.toLowerCase().includes(view.query)
  const underWay = view.underWay[node.branch]
  const dropped = view.dropped?.parent === node.branch ? view.dropped : undefined

  return (
    <Popover open={dropped !== undefined} onOpenChange={(open) => !open && view.dismiss()}>
      <PopoverAnchor asChild>
        <div
          ref={(element) => {
            dragRef(element)
            dropRef(element)
          }}
          className={cn(
            'flex h-12 items-center gap-2.5 rounded-md px-2',
            node.branch === view.open && 'bg-muted',
            isDropTarget && !isDragSource && 'bg-accent ring-1 ring-ring',
            (isDragSource || !found || underWay) && 'opacity-50',
          )}
        >
          {movable ? (
            <button
              type="button"
              ref={handleRef}
              aria-label={`Move ${node.branch} under another branch`}
              className="-ml-1 cursor-grab rounded-sm text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
            >
              <GripVertical className="size-4" />
            </button>
          ) : (
            <span className="w-4 shrink-0" />
          )}
          <span
            className={cn(
              'size-2.5 shrink-0 rounded-full',
              detail.virtual ? 'border-[1.5px] border-muted-foreground' : 'bg-foreground',
            )}
            aria-hidden="true"
          />
          {detail.virtual ? (
            <span className="font-medium text-muted-foreground">{node.branch}</span>
          ) : (
            <Link
              from="/r/$repo/worktrees"
              search={(search) => ({ ...search, worktree: node.branch })}
              className="font-medium underline-offset-4 hover:underline"
            >
              {node.branch}
            </Link>
          )}
          {underWay ? <UnderWay what={underWay} /> : <State node={node} />}
          <span className="truncate text-sm text-muted-foreground">{meta(detail)}</span>
          {node.pr && (
            <Badge variant="outline" asChild>
              <a href={node.pr.url} target="_blank" rel="noreferrer">
                #{node.pr.number} {node.pr.state}
              </a>
            </Badge>
          )}
          <span className="ml-auto flex items-center gap-3">
            {detail.services && detail.services.length > 0 && (
              <Glyph icon={CircleDot} tone="ok" className="text-sm">
                {detail.services.length} up
              </Glyph>
            )}
            {node.action && <ActionButton node={node} />}
          </span>
        </div>
      </PopoverAnchor>
      {dropped && (
        <PopoverContent align="start" className="flex w-[30rem] flex-col gap-3">
          <PopoverHeader>
            <PopoverTitle>
              Change the parent of {dropped.branch} to {dropped.parent}
            </PopoverTitle>
            <PopoverDescription>
              Reparent isn't in the web interface yet. Run these in a terminal: the first records the new parent, the
              second rebases onto it.
            </PopoverDescription>
          </PopoverHeader>
          <CommandLine followUp={reparent(dropped.branch, dropped.parent)} />
          <CommandLine followUp={sync(dropped.branch)} />
        </PopoverContent>
      )}
    </Popover>
  )
}

function State({ node }: { node: Node }) {
  const detail = node.detail
  if (detail.virtual) return <span className="text-sm text-muted-foreground">branch without a worktree</span>
  if (detail.missing)
    return (
      <Glyph icon={CircleSlash} tone="failed">
        missing
      </Glyph>
    )
  if (detail.rebasing)
    return (
      <Glyph icon={TriangleAlert} tone="attention">
        rebasing
      </Glyph>
    )
  if (node.action?.command === 'clean')
    return (
      <Glyph icon={GitMerge} tone="muted">
        merged
      </Glyph>
    )
  if (detail.dirty)
    return (
      <Glyph icon={CircleDashed} tone="attention">
        changed
      </Glyph>
    )
  return (
    <Glyph icon={CircleCheck} tone="ok">
      clean
    </Glyph>
  )
}

function meta(detail: Node['detail']): string {
  if (detail.main) return 'main worktree'
  if (detail.needs_sync && detail.behind) return `${detail.behind} behind ${detail.parent}`
  return detail.ahead ? `${detail.ahead} ahead` : ''
}

const capitalized = (word: string) => word.charAt(0).toUpperCase() + word.slice(1)

function ActionButton({ node }: { node: Node }) {
  const action = node.action
  if (!action) return null
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm" className="data-[state=open]:bg-accent">
          {action.command === 'go' ? 'Go to the worktree' : capitalized(action.command ?? 'open')}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="flex w-96 flex-col gap-3">
        <PopoverHeader>
          <PopoverTitle>{sentence(action.message)}</PopoverTitle>
          <PopoverDescription>Not in the web interface yet: copy this into a terminal.</PopoverDescription>
        </PopoverHeader>
        <CommandLine followUp={action} />
      </PopoverContent>
    </Popover>
  )
}
