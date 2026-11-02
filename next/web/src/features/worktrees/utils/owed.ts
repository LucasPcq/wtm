import type { TeardownDebt } from '@/api/types.gen'
import type { OutcomeRow } from '@/features/worktrees/utils/outcome'
import { sentence } from '@/features/worktrees/utils/sentence'

type Item = {
  subject: string
  status: string
  error?: { message: string } | undefined
  detail?: object | undefined
}

const owing = (detail: object): detail is { owed: TeardownDebt } => 'owed' in detail

// an item of the apply that is a drop owed the run planned to pay, rather than one of its worktrees
export const owedOf = ({ detail }: Item) => (detail && owing(detail) ? detail.owed : undefined)

// a drop owed as a row of the outcome, said from what became of it
export function owedRow(item: Item): OutcomeRow {
  const debt = owedOf(item)
  const base = { key: `owed-${item.subject}`, title: item.subject }
  if (item.status === 'done')
    return { ...base, tone: 'ok', message: `Dropped from ${debt?.job}, owed since ${debt?.worktree} was removed` }
  if (item.status === 'failed') return { ...base, tone: 'failed', message: item.error && sentence(item.error.message) }
  return { ...base, tone: 'muted', message: 'Not dropped: still owed' }
}
