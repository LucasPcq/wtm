import { type ReactElement, type ReactNode, useState } from 'react'
import { ContextMenu, ContextMenuContent, ContextMenuTrigger } from '@/components/ui/context-menu'
import { ActionItems } from '@/features/worktrees/components/action-items'
import { MoreMenu } from '@/features/worktrees/components/more-menu'
import type { Subject } from '@/features/worktrees/utils/actions'

// the row is the trigger of its context menu and draws its "⋯" where it wants it; a row under way offers neither
export function RowMenu({
  repo,
  subject,
  rows,
  busy,
  children,
}: {
  repo: string
  subject: string
  rows: Subject[]
  busy: boolean
  children: (more: ReactNode) => ReactElement
}) {
  const [open, setOpen] = useState(false)
  const [context, setContext] = useState(false)
  // a row that goes under way closes its menus for good: they would open again once the run ends
  if (busy && (open || context)) {
    setOpen(false)
    setContext(false)
  }

  const more = (
    <MoreMenu
      repo={repo}
      subject={subject}
      rows={rows}
      open={open}
      onOpenChange={setOpen}
      disabled={busy}
      className="opacity-0 group-focus-within/row:opacity-100 group-hover/row:opacity-100 focus-visible:opacity-100 data-[state=open]:opacity-100"
    />
  )

  return (
    <ContextMenu open={context && !busy} onOpenChange={setContext}>
      <ContextMenuTrigger
        asChild
        disabled={busy}
        // the row its menu acts on keeps its hover look while the menu is open
        data-menu-open={open || context || undefined}
        // the keyboard's menu key opens the "⋯" menu, anchored on the row rather than where a pointer would be
        onKeyDown={(event) => {
          if (busy || (event.key !== 'ContextMenu' && !(event.shiftKey && event.key === 'F10'))) return
          event.preventDefault()
          setOpen(true)
        }}
      >
        {children(more)}
      </ContextMenuTrigger>
      <ContextMenuContent className="w-64">
        <ActionItems kind="context" repo={repo} rows={rows} />
      </ContextMenuContent>
    </ContextMenu>
  )
}
