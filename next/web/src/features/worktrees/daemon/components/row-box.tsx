import { TriangleAlert } from 'lucide-react'
import { Item, ItemContent, ItemDescription, ItemTitle } from '@/components/ui/item'
import { Glyph } from '@/features/worktrees/components/glyph'
import { ItemBox } from '@/features/worktrees/components/item-box'
import type { Row } from '@/features/worktrees/daemon/utils/rows'

export function RowBox({ rows, label }: { rows: Row[]; label?: string }) {
  return (
    <ItemBox label={label}>
      {rows.map((row) => (
        <Item key={row.key} size="sm">
          <ItemContent>
            <ItemTitle>{row.title}</ItemTitle>
            {row.lines.map((line) => (
              <ItemDescription key={line}>{line}</ItemDescription>
            ))}
            {row.warning && (
              <Glyph icon={TriangleAlert} tone="attention" className="text-sm">
                {row.warning}
              </Glyph>
            )}
          </ItemContent>
        </Item>
      ))}
    </ItemBox>
  )
}
