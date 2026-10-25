import { LoaderCircle } from 'lucide-react'
import { Glyph } from './glyph'

// Under way is what an operation of this page does to a worktree, its row dimmed until it ends.
export function UnderWay({ what }: { what: string }) {
  return (
    <Glyph icon={LoaderCircle} tone="busy" className="text-muted-foreground">
      {what}
    </Glyph>
  )
}
