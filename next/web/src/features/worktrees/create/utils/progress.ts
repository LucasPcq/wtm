import type { CreatePlan, DomainPhase } from '@/api/types.gen'
import { todo, type Unit } from '@/features/worktrees/utils/progress'

// a branch both fast-forwarded and opened (checked out behind origin) is one unit, its fast-forward first
export function planned(plan: CreatePlan): Unit[] {
  const phases = new Map<string, DomainPhase[]>()
  for (const subject of plan.fast_forward ?? []) phases.set(subject, ['fast_forward'])
  for (const { branch, phases: opened } of plan.branches ?? [])
    if (opened && opened.length > 0) phases.set(branch, [...(phases.get(branch) ?? []), ...opened])

  return [...phases].map(([subject, of]) => todo(subject, of))
}
