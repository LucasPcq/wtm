import type { Read } from '@/api/queries/commands'
import type { DomainError, DomainFollowUp, DomainItem, ListDetail, ListPR, WorktreeCondition } from '@/api/types.gen'
import { overlaid } from '@/features/worktrees/shared/utils/condition'

export type Row = {
  subject: string
  status: DomainItem['status']
  detail: ListDetail | undefined
  error: DomainError | undefined
  pr: ListPR | null | undefined
  merged: boolean | undefined
  condition: WorktreeCondition
  actions: DomainFollowUp[]
  underWay: string | undefined
  readAt: number
}

export function rowsOf({
  live,
  github,
  readAt,
  underWay,
}: {
  live: Read<'list'>
  github: Read<'list'> | undefined
  readAt: number
  underWay: Record<string, string>
}): Row[] {
  const asked = new Map(github?.items.map((item) => [item.subject, item.detail]))

  return live.items.map(({ subject, status, detail, error }) => {
    const read = asked.get(subject)
    const shown = detail && overlaid(detail, read)
    return {
      subject,
      status,
      detail,
      error,
      pr: read?.pr,
      merged: read?.merged,
      // a worktree not read has no condition of its own: it reads as unreadable wherever its condition shows
      condition: shown?.condition ?? 'unreadable',
      actions: shown?.actions ?? [],
      underWay: underWay[subject],
      readAt,
    }
  })
}
