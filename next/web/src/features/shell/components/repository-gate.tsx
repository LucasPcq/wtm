import { useQuery } from '@tanstack/react-query'
import { FolderX, Plus } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { reposQuery } from '@/api/queries/repos'
import { Centered } from '@/components/centered'
import { StatePage } from '@/components/state-page'
import { Button } from '@/components/ui/button'
import { AddRepository } from '@/features/shell/components/add-repository'

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
  const [adding, setAdding] = useState(false)
  const known = repos.data?.find((candidate) => candidate.id === repo)

  if (known && !known.set_up) return notSetUp(known.root)
  if (!repos.data || known) return children

  return (
    <Centered>
      <StatePage
        icon={<FolderX />}
        title="This repository isn't here"
        action={
          <>
            <Button onClick={() => setAdding(true)}>
              <Plus />
              Add a repository
            </Button>
            <AddRepository open={adding} onOpenChange={setAdding} />
          </>
        }
      >
        Its address points to a repository that moved, or whose settings are gone. Pick one in the sidebar, or add it
        again by its path.
      </StatePage>
    </Centered>
  )
}
