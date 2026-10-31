import { HttpError } from '@/api/fetch'
import { Button } from '@/components/ui/button'
import { Alert } from '@/features/worktrees/components/alert'
import { CopyCommand } from '@/features/worktrees/create/components/copy-command'
import { unavailableLine } from '@/features/worktrees/create/utils/checkout-words'
import { sentence } from '@/features/worktrees/utils/sentence'

// GitHub that cannot be asked has its words and a way back to a new branch; any other failure says the server's
export function FormFailure({ error, onNewBranch }: { error: Error; onNewBranch: () => void }) {
  const refusal = error instanceof HttpError ? error.refusal : undefined
  const github = unavailableLine(refusal, error.message)

  if (!github) return <Alert tone="red" description={sentence(error.message)} />

  return (
    <Alert
      tone="amber"
      title={github.title}
      description={github.description}
      action={
        <Button variant="outline" size="sm" onClick={onNewBranch}>
          New branch
        </Button>
      }
      below={github.command && <CopyCommand text={github.command} />}
    />
  )
}
