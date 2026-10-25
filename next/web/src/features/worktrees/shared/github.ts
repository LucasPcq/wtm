import type { DomainWarning, WorktreePullRequestState } from '@/api/types.gen'

// unavailable is why GitHub could not be asked, as a read's warnings say, none when it answered.
export const unavailable = (warnings: DomainWarning[] | undefined) =>
  warnings?.find((warning) => warning.code.endsWith('.prs_unavailable'))

// How a pull request's state reads; an open one is the usual case and says nothing beside its number.
export const prState: Record<WorktreePullRequestState, string | undefined> = {
  open: undefined,
  merged: 'merged',
  closed: 'closed',
}
