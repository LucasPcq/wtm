import { createFileRoute } from '@tanstack/react-router'
import { FolderSearch } from 'lucide-react'
import { StatePage } from '@/components/state-page'
import { Centered } from '@/components/centered'

export const Route = createFileRoute('/_shell/')({
  component: () => (
    <Centered>
      <StatePage icon={<FolderSearch />} title="Choose a repository" command="wtm-next ui">
        Pick one in the sidebar, or run this in a repository to open it here:
      </StatePage>
    </Centered>
  ),
})
