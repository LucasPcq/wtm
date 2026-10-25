import { useQuery } from '@tanstack/react-query'
import { FolderX } from 'lucide-react'
import type { ReactNode } from 'react'
import { reposQuery } from '@/api/queries/repos'
import { Centered } from '@/components/centered'
import { StatePage } from '@/components/state-page'

export function RepositoryGate({ repo, children }: { repo: string; children: ReactNode }) {
  const repos = useQuery(reposQuery)

  if (!repos.data || repos.data.some((known) => known.id === repo)) return children

  return (
    <Centered>
      <StatePage icon={<FolderX />} title="wtm doesn't know this repository" command="wtm-next init">
        Its address points to a repository that is no longer initialized for wtm, or that moved. Pick one in the
        sidebar, or run this in it:
      </StatePage>
    </Centered>
  )
}
