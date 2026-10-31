import { HttpError } from '@/api/fetch'
import { Button } from '@/components/ui/button'
import { Alert } from '@/features/worktrees/components/alert'
import { GitHubAlert } from '@/features/worktrees/components/github-alert'
import { sentence } from '@/features/worktrees/utils/sentence'

// GitHub that cannot be asked says so as everywhere, with checkout's way back to a new branch; any other failure says
// the server's message
export function FormFailure({ error, onNewBranch }: { error: Error; onNewBranch: () => void }) {
  const refusal = error instanceof HttpError ? error.refusal : undefined

  if (refusal?.code !== 'checkout.github_unavailable') return <Alert tone="red" description={sentence(error.message)} />

  return (
    <GitHubAlert
      fact={refusal}
      also="A new branch still works."
      action={
        <Button variant="outline" size="sm" onClick={onNewBranch}>
          New branch
        </Button>
      }
    />
  )
}
