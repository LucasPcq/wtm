import { useQuery } from '@tanstack/react-query'
import { FolderX } from 'lucide-react'
import type { ReactNode } from 'react'
import { reposQuery } from '@/api/queries/repos'
import { Centered } from '@/components/centered'
import { StatePage } from '@/components/state-page'

// notSetUp is the page of a repository the web serves but wtm hasn't set up, from its main worktree
export function RepositoryGate({
  repo,
  notSetUp,
  children,
}: {
  repo: string
  notSetUp: (root: string) => ReactNode
  children: ReactNode
}) {
  const repos = useQuery(reposQuery)
  const known = repos.data?.find((candidate) => candidate.id === repo)

  if (known && !known.set_up) return notSetUp(known.root)
  if (!repos.data || known) return children

  return (
    <Centered>
      <StatePage icon={<FolderX />} title="wtm doesn't know this repository" command="wtm-next init">
        Its address points to a repository that is no longer initialized for wtm, or that moved. Pick one in the
        sidebar, or run this in it:
      </StatePage>
    </Centered>
  )
}
