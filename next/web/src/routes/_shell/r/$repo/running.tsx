import { createFileRoute } from '@tanstack/react-router'
import { RunningPage } from '@/features/worktrees/running/running-page'

export const Route = createFileRoute('/_shell/r/$repo/running')({ component: Running })

function Running() {
  return <RunningPage repo={Route.useParams().repo} />
}
