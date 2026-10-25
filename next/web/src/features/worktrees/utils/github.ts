import type { DomainWarning, WorktreePullRequestState } from '@/api/types.gen'

export const unavailable = (warnings: DomainWarning[] | undefined) =>
  warnings?.find((warning) => warning.code.endsWith('.prs_unavailable'))

// an open pull request, the usual case, says nothing beside its number
export const prState: Record<WorktreePullRequestState, string | undefined> = {
  open: undefined,
  merged: 'merged',
  closed: 'closed',
}
