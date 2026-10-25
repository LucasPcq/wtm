import { CircleCheck, CircleX, LoaderCircle } from 'lucide-react'
import type { WorktreeChecks } from '@/api/types.gen'
import { Glyph } from '@/features/worktrees/components/glyph'

export function Checks({ checks: { failed, pending, passed } }: { checks: WorktreeChecks }) {
  if (failed > 0)
    return (
      <Glyph icon={CircleX} tone="failed">
        {failed} checks failed
      </Glyph>
    )

  if (pending > 0)
    return (
      <Glyph icon={LoaderCircle} tone="attention">
        {pending} checks pending
      </Glyph>
    )

  return (
    <Glyph icon={CircleCheck} tone="ok">
      {passed} checks passed
    </Glyph>
  )
}
