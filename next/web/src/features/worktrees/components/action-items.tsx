import { Copy } from 'lucide-react'
import { Fragment } from 'react'
import { ContextMenuItem, ContextMenuLabel, ContextMenuSeparator } from '@/components/ui/context-menu'
import { DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator } from '@/components/ui/dropdown-menu'
import { MenuEntry } from '@/features/worktrees/components/menu-entry'
import { useRunEntry } from '@/features/worktrees/hooks/use-run-entry'
import { menu, menuActions, type Section, type Subject, selectionEntryOf } from '@/features/worktrees/utils/actions'

type MenuKind = 'context' | 'dropdown'

const parts = {
  context: { Item: ContextMenuItem, Label: ContextMenuLabel, Separator: ContextMenuSeparator },
  dropdown: { Item: DropdownMenuItem, Label: DropdownMenuLabel, Separator: DropdownMenuSeparator },
} satisfies Record<MenuKind, unknown>

const sections: Section[] = ['Branch', 'Run', 'Open', 'Remove']

export function ActionItems({ kind, repo, rows }: { kind: MenuKind; repo: string; rows: Subject[] }) {
  const run = useRunEntry(repo)
  const { Item, Label, Separator } = parts[kind]

  return (
    <>
      <Label className="truncate font-normal">
        {rows.length === 1 ? rows[0]?.subject : `${rows.length} selected worktrees`}
      </Label>
      {sections.map((section, i) => (
        <Fragment key={section}>
          {i > 0 && <Separator />}
          {section !== 'Remove' && <Label>{section}</Label>}
          {menuActions
            .filter((action) => menu[action].section === section)
            .map((action) => {
              const entry = selectionEntryOf(rows, action)
              const { label, icon, runs } = menu[action]
              return (
                <MenuEntry
                  key={action}
                  Item={Item}
                  icon={icon}
                  label={label}
                  why={'disabled' in entry ? entry.disabled : undefined}
                  destructive={section === 'Remove'}
                  onSelect={() => run(entry, rows)}
                >
                  {runs === 'copy' && <Copy aria-label="copies its command" className="text-muted-foreground" />}
                </MenuEntry>
              )
            })}
        </Fragment>
      ))}
    </>
  )
}
