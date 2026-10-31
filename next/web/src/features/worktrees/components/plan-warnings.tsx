import type { DomainWarning } from '@/api/types.gen'
import { Alert } from '@/features/worktrees/components/alert'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { GitHubAlert } from '@/features/worktrees/components/github-alert'
import { isUnavailable } from '@/features/worktrees/utils/github'
import { sentence } from '@/features/worktrees/utils/sentence'

export function PlanWarnings({ warnings }: { warnings: DomainWarning[] }) {
  if (warnings.length === 0) return null

  return (
    <div className="flex flex-col gap-2">
      {warnings.map((warning) =>
        isUnavailable(warning) ? (
          <GitHubAlert key={warning.code} fact={warning} />
        ) : (
          <Alert
            key={`${warning.code}-${warning.message}`}
            tone="amber"
            description={sentence(warning.message)}
            action={warning.follow_up && <CommandLine followUp={warning.follow_up} look="inline" />}
          />
        ),
      )}
    </div>
  )
}
