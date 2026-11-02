import type { PruneCandidate, TeardownDebt, TeardownMove } from '@/api/types.gen'
import { DataChoice } from '@/features/worktrees/components/data-choice'
import { ItemList } from '@/features/worktrees/components/item-list'
import { type Left, LeftItem } from '@/features/worktrees/components/left-item'
import { RemovalDetails } from '@/features/worktrees/components/removal-details'
import { RemovalItem } from '@/features/worktrees/components/removal-item'
import { OwedList } from '@/features/worktrees/prune/components/owed-list'
import { ReasonLine } from '@/features/worktrees/prune/components/reason-line'
import { downOf } from '@/features/worktrees/utils/removal'

export function PruneConfirm({
  pruning,
  left,
  moves,
  reparent,
  owed,
  dropData,
  onDropData,
}: {
  pruning: PruneCandidate[]
  left: Left[]
  moves: TeardownMove[]
  reparent: boolean
  owed: TeardownDebt[]
  dropData: boolean
  onDropData: (on: boolean) => void
}) {
  const down = downOf(pruning)

  return (
    <>
      {(pruning.length > 0 || left.length > 0) && (
        <ItemList
          label="Worktrees"
          left={left.map(({ worktree, why }) => <LeftItem key={worktree} worktree={worktree} why={why} />)}
        >
          {pruning.map((candidate) => (
            <RemovalItem
              key={candidate.worktree}
              removal={candidate}
              reason={<ReasonLine candidate={candidate} />}
              moves={moves.filter((move) => move.from === candidate.branch)}
              reparent={reparent}
            />
          ))}
        </ItemList>
      )}
      {owed.length > 0 && <OwedList owed={owed} />}
      {pruning.length > 0 && <RemovalDetails removals={pruning} />}
      {down.length > 0 && <DataChoice down={down} dropData={dropData} onDropData={onDropData} />}
    </>
  )
}
