import { CircleX } from 'lucide-react'
import type { DomainError } from '@/api/types.gen'
import { lockWords } from '@/features/worktrees/exec/utils/words'

export function LockAlert({ refusal, live }: { refusal: DomainError; live: boolean }) {
  const { title, description } = lockWords(refusal, live)

  return (
    <div
      role="alert"
      className="flex items-start gap-3 rounded-lg border border-destructive/35 bg-destructive/8 p-3 text-sm"
    >
      <CircleX className="mt-0.5 size-4 shrink-0 text-destructive" aria-hidden="true" />
      <div className="flex min-w-0 flex-col gap-1">
        <span className="font-medium">{title}</span>
        <span className="text-muted-foreground">{description}</span>
      </div>
    </div>
  )
}
