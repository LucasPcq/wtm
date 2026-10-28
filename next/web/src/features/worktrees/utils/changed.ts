import type { DomainError, DomainWarning } from '@/api/types.gen'
import { listed } from '@/features/worktrees/utils/listed'

// refused at its apply as unsafe, a worktree changed since the plan: nothing ran, and the wizard asks again on fresh
// rows, where the changed one shows its risk, with this notice above them
export function changedSincePlan(
  { progress, running }: { progress: { envelope?: { error?: DomainError } } | undefined; running: boolean },
  { code, past }: { code: string; past: string },
): DomainWarning | undefined {
  const refusal = running ? undefined : progress?.envelope?.error
  if (refusal?.code !== code) return undefined
  const names = [...new Set((refusal.fields ?? []).flatMap((field) => field.params?.value ?? []))]
  return {
    code,
    message: `${listed(names) || 'A worktree'} changed since the plan, so nothing was ${past}. Check the worktrees again.`,
  }
}
