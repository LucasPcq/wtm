import { Link } from '@tanstack/react-router'
import { GitBranchPlus, SearchX } from 'lucide-react'
import { StatePage } from '@/components/state-page'
import { Button } from '@/components/ui/button'

export function OnlyMain() {
  return (
    <div className="py-16">
      <StatePage icon={<GitBranchPlus />} title="No other worktree yet" command="wtm create feat/my-branch">
        Create one per branch you work on: each gets its own folder, .env and ports, so they run side by side.
      </StatePage>
    </div>
  )
}

export function NoMatch() {
  return (
    <div className="py-16">
      <StatePage icon={<SearchX />} title="No worktree matches">
        Nothing here matches the search and the filter.
      </StatePage>
      <div className="flex justify-center">
        <Button variant="outline" asChild>
          <Link from="/r/$repo/worktrees" search={(search) => ({ ...search, q: undefined, filter: undefined })}>
            Clear the search and filters
          </Link>
        </Button>
      </div>
    </div>
  )
}
