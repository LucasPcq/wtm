import { FolderSearch } from 'lucide-react'
import { Centered } from '@/components/centered'
import { StatePage } from '@/components/state-page'

export function NoRepository() {
  return (
    <Centered>
      <StatePage icon={<FolderSearch />} title="Choose a repository" command="wtm-next ui">
        Pick one in the sidebar, or run this in a repository to open it here:
      </StatePage>
    </Centered>
  )
}
