import { Info } from 'lucide-react'
import type { DomainWarning } from '@/api/types.gen'
import { sentence } from '@/features/worktrees/utils/sentence'

export function Notes({ warnings }: { warnings: DomainWarning[] }) {
  if (warnings.length === 0) return null

  return (
    <ul className="flex flex-col gap-1 px-5 py-3 text-xs text-muted-foreground">
      {warnings.map(({ code, message }) => (
        <li key={code} className="flex items-center gap-1.5">
          <Info className="size-3.5 shrink-0" aria-hidden="true" />
          {sentence(message)}
        </li>
      ))}
    </ul>
  )
}
