import type { DomainWarning } from '@/api/types.gen'
import { Alert } from '@/features/worktrees/components/alert'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { sentence } from '@/features/worktrees/utils/sentence'

export function PlanWarnings({ warnings }: { warnings: DomainWarning[] }) {
  if (warnings.length === 0) return null

  return (
    <div className="flex flex-col gap-2">
      {warnings.map((warning) => (
        <Alert
          key={`${warning.code}-${warning.message}`}
          tone="amber"
          description={sentence(warning.message)}
          action={warning.follow_up && <CommandLine followUp={warning.follow_up} look="inline" />}
        />
      ))}
    </div>
  )
}
