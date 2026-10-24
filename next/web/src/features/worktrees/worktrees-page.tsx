import { useQuery } from '@tanstack/react-query'
import { CircleAlert, Plus } from 'lucide-react'
import { readQuery } from '@/api/queries/commands'
import { reposQuery } from '@/api/queries/repos'
import { Centered } from '@/components/centered'
import { PageHeader } from '@/components/page-header'
import { StatePage } from '@/components/state-page'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { repoName } from '@/lib/repo'

// Worktrees is the screen of a repository's worktrees; its table arrives with LUC-360, the shell draws its frame.
export function WorktreesPage({ repo }: { repo: string }) {
  const list = useQuery(readQuery(repo, 'list'))
  const root = useQuery(reposQuery).data?.find((known) => known.id === repo)?.root
  const count = list.data?.items?.length

  return (
    <>
      <PageHeader
        title="Worktrees"
        description={
          count !== undefined && root && `${count} ${count === 1 ? 'worktree' : 'worktrees'} in ${repoName(root)}`
        }
        action={
          <Button disabled>
            <Plus />
            Create worktree
          </Button>
        }
      />
      {list.isError && (
        <Centered>
          <StatePage icon={<CircleAlert />} title="wtm couldn't read the worktrees">
            {list.error.message}
          </StatePage>
        </Centered>
      )}
      <div className="flex flex-col gap-3 p-8" aria-busy={list.isPending}>
        {list.isPending &&
          [260, 195, 213, 156, 182].map((width) => (
            <div key={width} className="flex h-11 items-center gap-6 border-b px-3">
              <Skeleton className="h-3.5" style={{ width }} />
              <Skeleton className="h-3.5 w-[90px]" />
              <Skeleton className="h-3.5 w-[60px]" />
            </div>
          ))}
      </div>
    </>
  )
}
