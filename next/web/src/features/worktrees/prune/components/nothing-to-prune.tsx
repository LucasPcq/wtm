import { GitBranchMinus } from 'lucide-react'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'

export function NothingToPrune() {
  return (
    <Empty className="p-6">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <GitBranchMinus />
        </EmptyMedia>
        <EmptyTitle>Nothing to prune</EmptyTitle>
        <EmptyDescription>
          A worktree can be pruned once its pull request is merged or closed, or origin deleted its branch, and nothing
          would be lost.
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}
