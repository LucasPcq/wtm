import { useDraggable, useDroppable } from '@dnd-kit/react'
import { Link } from '@tanstack/react-router'
import { CircleDot, GripVertical, LoaderCircle } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import {
  Popover,
  PopoverAnchor,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
} from '@/components/ui/popover'
import { searched } from '@/features/worktrees/list/utils/filters'
import { CommandLine } from '@/features/worktrees/shared/components/command-line'
import { Glyph } from '@/features/worktrees/shared/components/glyph'
import { conditionLook } from '@/features/worktrees/shared/utils/condition'
import { ActionButton } from '@/features/worktrees/tree/components/action-button'
import type { View } from '@/features/worktrees/tree/components/branch'
import { type Node, reparent, sync } from '@/features/worktrees/tree/utils/nodes'
import { cn } from '@/lib/utils'

export function NodeRow({ node, view }: { node: Node; view: View }) {
  const { branch, detail, pr, action, condition } = node
  const movable = !detail.main && !detail.virtual
  const { ref: dragRef, handleRef, isDragSource } = useDraggable({ id: branch, disabled: !movable })
  const { ref: dropRef, isDropTarget } = useDroppable({ id: branch })

  const found = searched(branch, view.query)
  const underWay = view.underWay[branch]
  const dropped = view.dropped?.parent === branch ? view.dropped : undefined
  const look = conditionLook[condition]

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
    if (condition === 'no_worktree') return <span className="text-sm text-muted-foreground">{look.label}</span>
    return (
      <Glyph icon={look.icon} tone={look.tone}>
        {look.label}
      </Glyph>
    )
  }

  return (
    <Popover open={dropped !== undefined} onOpenChange={(open) => !open && view.dismiss()}>
      <PopoverAnchor asChild>
        <div
          ref={(element) => {
            dragRef(element)
            dropRef(element)
          }}
          className={cn(
            'relative flex h-12 items-center gap-2.5 rounded-md px-2 hover:bg-muted/50',
            branch === view.open && 'bg-muted',
            isDropTarget && !isDragSource && 'bg-accent ring-1 ring-ring',
            (isDragSource || !found || underWay) && 'opacity-50',
          )}
        >
          {movable ? (
            <button
              type="button"
              ref={handleRef}
              aria-label={`Move ${branch} under another branch`}
              className="relative z-10 -ml-1 cursor-grab rounded-sm text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
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
            {detail.services && detail.services.length > 0 && (
              <Glyph icon={CircleDot} tone="ok" className="text-sm">
                {detail.services.length} up
              </Glyph>
            )}
            {action && <ActionButton action={action} />}
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
