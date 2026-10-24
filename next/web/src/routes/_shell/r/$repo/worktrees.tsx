import { createFileRoute } from '@tanstack/react-router'
import { WorktreesPage } from '@/features/worktrees/worktrees-page'

export const Route = createFileRoute('/_shell/r/$repo/worktrees')({ component: Worktrees })

function Worktrees() {
  return <WorktreesPage repo={Route.useParams().repo} />
}
