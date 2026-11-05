import type { RunjobsJobPlan, RunupPlan } from '@/api/types.gen'

// a job of a start's plan, by its worktree and its name
export const jobPlan = (plan: RunupPlan | undefined, worktree: string, name: string): RunjobsJobPlan | undefined =>
  plan?.worktrees?.find((target) => target.worktree === worktree)?.jobs?.find((job) => job.name === name)
