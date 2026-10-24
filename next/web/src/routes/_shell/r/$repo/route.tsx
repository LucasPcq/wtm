import { useQuery } from '@tanstack/react-query'
import { Outlet, createFileRoute } from '@tanstack/react-router'
import { FolderX } from 'lucide-react'
import { reposQuery } from '@/api/queries'
import { StatePage } from '@/components/state-page'
import { Centered } from '@/components/centered'

export const Route = createFileRoute('/_shell/r/$repo')({
  component: Repository,
})

// Repository serves the screens of the repository its id names, once the server knows it.
function Repository() {
  const { repo } = Route.useParams()
  const repos = useQuery(reposQuery)
  if (repos.data && !repos.data.some((known) => known.id === repo)) {
    return (
      <Centered>
        <StatePage icon={<FolderX />} title="wtm doesn't know this repository" command="wtm-next init">
          Its address points to a repository that is no longer initialized for wtm, or that moved. Pick one in the
          sidebar, or run this in it:
        </StatePage>
      </Centered>
    )
  }
  return <Outlet />
}
