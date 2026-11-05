import { createFileRoute } from '@tanstack/react-router'
import { RunningPage } from '@/features/worktrees/running/running-page'
import { validateSearch } from '@/features/worktrees/running/utils/search'

export const Route = createFileRoute('/_shell/r/$repo/running')({ component: Running, validateSearch })

function Running() {
  return <RunningPage repo={Route.useParams().repo} search={Route.useSearch()} />
}
