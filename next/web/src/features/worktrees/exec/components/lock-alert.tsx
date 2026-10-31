import type { DomainError } from '@/api/types.gen'
import { Alert } from '@/features/worktrees/components/alert'
import { lockWords } from '@/features/worktrees/exec/utils/words'

export function LockAlert({ refusal, live }: { refusal: DomainError; live: boolean }) {
  const { title, description } = lockWords(refusal, live)

  return <Alert tone="red" title={title} description={description} />
}
