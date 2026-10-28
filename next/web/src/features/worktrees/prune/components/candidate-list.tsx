import { useState } from 'react'
import type { PruneCandidate } from '@/api/types.gen'
import { ItemList } from '@/features/worktrees/components/item-list'
import { type Left, LeftItem } from '@/features/worktrees/components/left-item'
import { ListHeader } from '@/features/worktrees/components/list-header'
import { RemovalItem } from '@/features/worktrees/components/removal-item'
import { ReasonLine } from '@/features/worktrees/prune/components/reason-line'
import { searched } from '@/features/worktrees/utils/filters'
import { unsafe } from '@/features/worktrees/utils/removal'

export function CandidateList({
  candidates,
  isChecked,
  onCheck,
  left,
}: {
  candidates: PruneCandidate[]
  isChecked: (candidate: PruneCandidate) => boolean
  onCheck: (some: PruneCandidate[], on: boolean) => void
  left: Left[]
}) {
  const [query, setQuery] = useState('')

  const visible = candidates.filter((candidate) => searched(candidate.worktree, query))

  return (
    <ItemList
      label="Worktrees"
      header={
        <ListHeader
          total={candidates.length}
          visible={visible.map((candidate) => candidate.worktree)}
          bulk={visible.filter((candidate) => !unsafe(candidate)).map((candidate) => candidate.worktree)}
          checked={candidates.filter(isChecked).map((candidate) => candidate.worktree)}
          query={query}
          onQuery={setQuery}
          onCheck={(names, on) =>
            onCheck(
              visible.filter((candidate) => names.includes(candidate.worktree)),
              on,
            )
          }
        />
      }
      left={left.map(({ worktree, why }) => <LeftItem key={worktree} worktree={worktree} why={why} />)}
    >
      {visible.map((candidate) => (
        <RemovalItem
          key={candidate.worktree}
          removal={candidate}
          verb="Prune"
          reason={<ReasonLine candidate={candidate} />}
          checked={isChecked(candidate)}
          onCheck={(on) => onCheck([candidate], on)}
        />
      ))}
    </ItemList>
  )
}
