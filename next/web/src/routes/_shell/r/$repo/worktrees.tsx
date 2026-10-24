import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { readQuery, reposQuery } from '@/api/queries'
import { PageHeader } from '@/components/page-header'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { repoName } from '@/lib/repo'

export const Route = createFileRoute('/_shell/r/$repo/worktrees')({
  component: Worktrees,
})

// Worktrees is the screen of a repository's worktrees; its table arrives with LUC-360, the shell draws its frame.
function Worktrees() {
  const { repo } = Route.useParams()
  const list = useQuery(readQuery(repo, 'list'))
  const root = useQuery(reposQuery).data?.find((known) => known.id === repo)?.root

  return (
    <>
      <PageHeader
        title="Worktrees"
        description={list.data?.items && root && `${list.data.items.length} worktrees in ${repoName(root)}`}
        action={
          <Button disabled>
            <Plus />
            Create worktree
          </Button>
        }
      />
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
