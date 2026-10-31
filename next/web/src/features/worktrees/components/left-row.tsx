import { CircleMinus, TriangleAlert } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Item, ItemTitle } from '@/components/ui/item'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { toneIcon } from '@/features/worktrees/components/glyph'
import type { OutcomeRow } from '@/features/worktrees/utils/outcome'

export function LeftRow({ row }: { row: OutcomeRow }) {
  const Icon = row.tone === 'attention' ? TriangleAlert : CircleMinus

  return (
    <Item role="listitem" aria-label={row.title} className="items-center gap-3 rounded-none px-4 py-3">
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <ItemTitle className="min-w-0">{row.title}</ItemTitle>
        <span className="flex items-center gap-1.5 text-xs">
          <Icon className={toneIcon({ tone: row.tone ?? 'muted' })} aria-hidden="true" />
          {row.message}
        </span>
      </div>
      {row.act && (
        <Button variant="outline" size="sm" onClick={row.act}>
          {row.actLabel}
        </Button>
      )}
      {row.followUp && !row.act && <CommandLine followUp={row.followUp} look="copy" />}
    </Item>
  )
}
