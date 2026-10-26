import type { Read } from '@/api/queries/commands'
import type {
  DomainError,
  DomainFollowUp,
  DomainItem,
  ListDetail,
  ListFinished,
  ListPR,
  WorktreeCondition,
} from '@/api/types.gen'
import { overlaid } from '@/features/worktrees/utils/condition'

export type Row = {
  subject: string
  status: DomainItem['status']
  detail: ListDetail | undefined
  error: DomainError | undefined
  pr: ListPR | null | undefined
  merged: boolean | undefined
  finished: ListFinished | null | undefined
  condition: WorktreeCondition | undefined
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
      finished: read?.finished,
      condition: shown?.condition,
      actions: shown?.actions ?? [],
      underWay: underWay[subject],
      readAt,
    }
  })
}

export const unread = ({ status }: Row) => (status === 'cancelled' ? 'not read' : 'unreadable')
