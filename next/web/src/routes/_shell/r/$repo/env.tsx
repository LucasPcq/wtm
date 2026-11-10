import { createFileRoute } from '@tanstack/react-router'
import { EnvFilesPage } from '@/features/worktrees/env-files/env-files-page'
import { validateSearch } from '@/features/worktrees/env-files/utils/search'

export const Route = createFileRoute('/_shell/r/$repo/env')({ component: EnvFiles, validateSearch })

function EnvFiles() {
  return <EnvFilesPage repo={Route.useParams().repo} search={Route.useSearch()} />
}
