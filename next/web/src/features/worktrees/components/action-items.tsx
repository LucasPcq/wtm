import { Copy } from 'lucide-react'
import { Fragment } from 'react'
import { ContextMenuItem, ContextMenuLabel, ContextMenuSeparator } from '@/components/ui/context-menu'
import { DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator } from '@/components/ui/dropdown-menu'
import { useRunEntry } from '@/features/worktrees/hooks/use-run-entry'
import { menu, menuActions, type Section, type Subject, selectionEntryOf } from '@/features/worktrees/utils/actions'
import { cn } from '@/lib/utils'

type MenuKind = 'context' | 'dropdown'

const parts = {
  context: { Item: ContextMenuItem, Label: ContextMenuLabel, Separator: ContextMenuSeparator },
  dropdown: { Item: DropdownMenuItem, Label: DropdownMenuLabel, Separator: DropdownMenuSeparator },
} satisfies Record<MenuKind, unknown>

const sections: Section[] = ['Branch', 'Run', 'Open', 'Remove']

// a disabled item stays in place and says why on its second line; aria-disabled, not Radix's disabled, which the
// keyboard skips: the reason would be out of its reach
export function ActionItems({ kind, repo, rows }: { kind: MenuKind; repo: string; rows: Subject[] }) {
  const run = useRunEntry(repo)
  const { Item, Label, Separator } = parts[kind]

  return (
    <>
      {rows.length > 1 && <Label className="font-normal">{rows.length} selected worktrees</Label>}
      {sections.map((section, i) => (
        <Fragment key={section}>
          {i > 0 && <Separator />}
          {section !== 'Remove' && <Label>{section}</Label>}
          {menuActions
            .filter((action) => menu[action].section === section)
            .map((action) => {
              const entry = selectionEntryOf(rows, action)
              const { label, icon: Icon, runs } = menu[action]
              const off = 'disabled' in entry
              return (
                <Item
                  key={action}
                  aria-disabled={off || undefined}
                  variant={section === 'Remove' ? 'destructive' : 'default'}
                  className={cn(off && 'opacity-50')}
                  onSelect={(event) => {
                    if (off) return event.preventDefault()
                    run(entry, rows)
                  }}
                >
                  <Icon />
                  <span className="flex min-w-0 flex-1 flex-col">
                    {label}
                    {off && <span className="text-xs text-muted-foreground">{entry.disabled}</span>}
                  </span>
                  {runs === 'copy' && <Copy aria-label="copies its command" className="text-muted-foreground" />}
                </Item>
              )
            })}
        </Fragment>
      ))}
    </>
  )
}
