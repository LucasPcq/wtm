import type { DomainError } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Alert } from '@/features/worktrees/components/alert'
import { CopyCommand } from '@/features/worktrees/create/components/copy-command'
import { unavailableLine } from '@/features/worktrees/create/utils/checkout-words'

export function GitHubAlert({
  refusal,
  message,
  onNewBranch,
}: {
  refusal: DomainError | undefined
  message: string
  onNewBranch: () => void
}) {
  const { title, description, command } = unavailableLine(refusal, message)

  return (
    <Alert
      tone="amber"
      title={title}
      description={description}
      action={
        <Button variant="outline" size="sm" onClick={onNewBranch}>
          New branch
        </Button>
      }
      below={command && <CopyCommand text={command} />}
    />
  )
}
