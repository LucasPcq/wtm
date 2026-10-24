import { Outlet, createFileRoute } from '@tanstack/react-router'
import { RepositoryGate } from '@/features/shell/repository-gate'

export const Route = createFileRoute('/_shell/r/$repo')({ component: Repository })

function Repository() {
  return (
    <RepositoryGate repo={Route.useParams().repo}>
      <Outlet />
    </RepositoryGate>
  )
}
