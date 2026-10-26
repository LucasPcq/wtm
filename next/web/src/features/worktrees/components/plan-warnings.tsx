import { TriangleAlert } from 'lucide-react'
import type { DomainWarning } from '@/api/types.gen'
import { sentence } from '@/features/worktrees/utils/sentence'

export function PlanWarnings({ warnings }: { warnings: DomainWarning[] }) {
  if (warnings.length === 0) return null

  return (
    <ul className="flex flex-col gap-2 text-sm">
      {warnings.map((warning) => (
        <li key={`${warning.code}-${warning.message}`} className="flex items-start gap-3">
          <TriangleAlert className="mt-0.5 size-4 shrink-0 text-amber-500" aria-hidden="true" />
          {sentence(warning.message)}
        </li>
      ))}
    </ul>
  )
}
