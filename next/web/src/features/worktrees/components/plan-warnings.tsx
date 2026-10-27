import { TriangleAlert } from 'lucide-react'
import type { DomainWarning } from '@/api/types.gen'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { sentence } from '@/features/worktrees/utils/sentence'

export function PlanWarnings({ warnings }: { warnings: DomainWarning[] }) {
  if (warnings.length === 0) return null

  return (
    <ul className="flex flex-col gap-2 text-sm">
      {warnings.map((warning) => (
        <li
          key={`${warning.code}-${warning.message}`}
          role="alert"
          className="flex items-center gap-3 rounded-lg border border-amber-500/35 bg-amber-500/8 p-3"
        >
          <TriangleAlert className="size-4 shrink-0 text-amber-500" aria-hidden="true" />
          <span className="min-w-0 flex-1">{sentence(warning.message)}</span>
          {warning.follow_up && <CommandLine followUp={warning.follow_up} look="inline" />}
        </li>
      ))}
    </ul>
  )
}
