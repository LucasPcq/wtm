import type { Read } from '@/api/queries/commands'
import type { DomainError, DomainWarning, ListDetail, ListPR } from '@/api/types.gen'

// A Row is one worktree of the list as the screen shows it: the live read's detail, and what GitHub said of it the
// last time it was asked (its pull request, whether it merged it), none until then.
export type Row = {
  subject: string
  detail: ListDetail | undefined
  error: DomainError | undefined
  warnings: DomainWarning[]
  pr: ListPR | null | undefined
  merged: boolean | undefined
}

// rowsOf is each worktree of live with what github read of it, matched by subject; a worktree github has not seen
// (created since) has no pull request known yet.
export function rowsOf(live: Read<'list'>, github: Read<'list'> | undefined): Row[] {
  const asked = new Map(github?.items.map((item) => [item.subject, item.detail]))
  return live.items.map((item) => {
    const read = asked.get(item.subject)
    return {
      subject: item.subject,
      detail: item.detail,
      error: item.error,
      warnings: item.warnings ?? [],
      pr: read?.pr,
      merged: read?.merged,
    }
  })
}
