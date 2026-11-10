import { createFileRoute, Outlet } from '@tanstack/react-router'
import { RepositoryGate } from '@/features/shell/components/repository-gate'
import { SetUpGate } from '@/features/worktrees/set-up/set-up-gate'

// setup opens the set-up dialog of a repository not set up yet, as Add a repository leaves it
export const Route = createFileRoute('/_shell/r/$repo')({
  validateSearch: (search: Record<string, unknown>): { setup?: true | undefined } =>
    search.setup === true ? { setup: true } : {},
  component: Repository,
})

function Repository() {
  const repo = Route.useParams().repo
  const opened = Route.useSearch().setup === true

  return (
    <RepositoryGate repo={repo} notSetUp={(root) => <SetUpGate key={repo} repo={repo} root={root} opened={opened} />}>
      <Outlet />
    </RepositoryGate>
  )
}
