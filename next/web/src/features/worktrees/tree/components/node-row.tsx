import { Feedback } from '@dnd-kit/dom'
import { useDraggable, useDroppable } from '@dnd-kit/react'
import { Link } from '@tanstack/react-router'
import { Ban, CircleDot, GripVertical, LoaderCircle, TriangleAlert } from 'lucide-react'
import { type ReactNode, useCallback } from 'react'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Glyph } from '@/features/worktrees/components/glyph'
import { RowMenu } from '@/features/worktrees/components/row-menu'
import type { View } from '@/features/worktrees/tree/components/branch'
import type { Node } from '@/features/worktrees/tree/utils/nodes'
import { landedHere } from '@/features/worktrees/tree/utils/targets'
import { conditionLook } from '@/features/worktrees/utils/condition'
import { searched } from '@/features/worktrees/utils/filters'
import { cn } from '@/lib/utils'

export function NodeRow({ node, view }: { node: Node; view: View }) {
  const { branch, detail, pr, condition } = node
  const underWay = view.underWay[branch]
  const movable = view.movable.includes(branch)
  // a clone follows the pointer: the row stays in its place, dimmed, where it is dragged from
  const {
    ref: dragRef,
    handleRef,
    isDragSource,
  } = useDraggable({
    id: branch,
    disabled: !movable || underWay !== undefined,
    plugins: [Feedback.configure({ feedback: 'clone' })],
  })
  const { ref: dropRef, isDropTarget } = useDroppable({ id: branch })

  // stable while the row has not landed: dnd-kit's handle is registered once, and focus moves once the read draws the
  // row under its new parent, where the drop sent it
  const landed = landedHere(view.moving, { branch, parent: detail.parent })
  const { landed: done } = view
  const gripRef = useCallback(
    (element: HTMLButtonElement | null) => {
      handleRef(element)
      if (!element || !landed) return
      element.focus()
      done()
    },
    [handleRef, landed, done],
  )

  const found = searched(branch, view.query)
  const look = conditionLook[condition]
  const target = isDragSource ? undefined : view.target?.(node)
  const refused = isDropTarget && target?.kind === 'refused' ? target.why : undefined

  const meta = () => {
    if (detail.main) return 'main worktree'
    if (detail.needs_sync && detail.behind) return `${detail.behind} behind ${detail.parent}`
    return detail.ahead ? `${detail.ahead} ahead` : ''
  }

  const state = () => {
    if (underWay)
      return (
        <Glyph icon={LoaderCircle} tone="busy">
          {underWay}
        </Glyph>
      )
    if (detail.virtual) return <span className="text-sm text-muted-foreground">{look.label}</span>
    return (
      <>
        <Glyph icon={look.icon} tone={look.tone}>
          {look.label}
        </Glyph>
        {detail.parent_gone && (
          <Glyph icon={TriangleAlert} tone="attention">
            parent gone
          </Glyph>
        )}
      </>
    )
  }

  const offers = view.subjects.find((subject) => subject.subject === branch)?.offers

  const row = (more?: ReactNode) => (
    <div
      ref={(element) => {
        dragRef(element)
        dropRef(element)
      }}
      className={cn(
        'group/row relative flex h-12 items-center gap-2.5 rounded-md px-2 hover:bg-muted/50',
        branch === view.open ? 'bg-muted' : 'data-menu-open:bg-muted/50',
        isDropTarget && target?.kind === 'valid' && 'bg-accent ring-1 ring-ring',
        (isDragSource || !found || underWay) && 'opacity-50',
      )}
    >
      {movable ? (
        // kept while under way, so a keyboard drop keeps its focus
        <button
          type="button"
          ref={gripRef}
          aria-label={`Move ${branch} under another branch`}
          aria-disabled={underWay !== undefined}
          // where it may land is asked before the drag starts
          onPointerEnter={() => view.prefetch(branch)}
          onFocus={() => view.prefetch(branch)}
          className="relative z-10 -ml-1 cursor-grab rounded-sm text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring aria-disabled:cursor-not-allowed"
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
        <span className="font-medium text-muted-foreground">{branch}</span>
      ) : (
        // the whole row opens the detail; the grip and the buttons sit above the link
        <Link
          from="/r/$repo/worktrees"
          search={(search) => ({ ...search, worktree: branch })}
          className="rounded-sm font-medium outline-none after:absolute after:inset-0 focus-visible:ring-2 focus-visible:ring-ring"
        >
          {branch}
        </Link>
      )}
      {state()}
      <span className="truncate text-sm text-muted-foreground">{meta()}</span>
      {pr && (
        <Badge variant="outline" className="relative z-10" asChild>
          <a href={pr.url} target="_blank" rel="noreferrer">
            #{pr.number} {pr.state}
          </a>
        </Badge>
      )}
      <span className="relative z-10 ml-auto flex items-center gap-3">
        {target?.kind === 'parent' && <span className="text-sm text-muted-foreground">current parent</span>}
        {/* open while the dragged worktree is over it: a refused target says why before the drop */}
        <Tooltip open={refused !== undefined}>
          <TooltipTrigger asChild>
            <span className={refused ? undefined : 'hidden'}>
              <Ban className="size-4 text-muted-foreground" aria-hidden="true" />
            </span>
          </TooltipTrigger>
          <TooltipContent>{refused}</TooltipContent>
        </Tooltip>
        {detail.services && detail.services.length > 0 && (
          <Glyph icon={CircleDot} tone="ok" className="text-sm">
            {detail.services.length} up
          </Glyph>
        )}
        {more}
      </span>
    </div>
  )

  if (!offers) return row()
  return (
    <RowMenu repo={view.repo} subject={branch} rows={[{ subject: branch, offers }]} busy={underWay !== undefined}>
      {(more) => row(more)}
    </RowMenu>
  )
}
