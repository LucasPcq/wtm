import type { CreatePlan, DomainAnswered, DomainPhase } from '@/api/types.gen'
import type { Unit } from '@/features/worktrees/utils/progress'

export type Planned = { answers: DomainAnswered[]; fingerprint: string; plan: CreatePlan }

// a branch both fast-forwarded and opened (checked out behind origin) is one unit, its fast-forward first
export function planned(plan: CreatePlan): Unit[] {
  const phases = new Map<string, DomainPhase[]>()
  for (const subject of plan.fast_forward ?? []) phases.set(subject, ['fast_forward'])
  for (const { branch, phases: opened } of plan.branches ?? [])
    if (opened && opened.length > 0) phases.set(branch, [...(phases.get(branch) ?? []), ...opened])

  return [...phases].map(([subject, of]) => ({
    subject,
    phases: of.map((phase) => ({ phase, state: 'todo' as const })),
    lines: [],
  }))
}
