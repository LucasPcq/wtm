import { createFileRoute } from '@tanstack/react-router'
import { validateSearch } from '@/features/worktrees/search'
import { WorktreesPage } from '@/features/worktrees/worktrees-page'

export const Route = createFileRoute('/_shell/r/$repo/worktrees')({ validateSearch, component: Worktrees })

function Worktrees() {
  return <WorktreesPage repo={Route.useParams().repo} search={Route.useSearch()} />
}
