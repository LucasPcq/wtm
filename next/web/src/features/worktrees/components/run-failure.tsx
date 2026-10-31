import { Alert } from '@/features/worktrees/components/alert'

export function RunFailure({ message }: { message: string | undefined }) {
  if (!message) return null

  return <Alert tone="red" description={message} />
}
