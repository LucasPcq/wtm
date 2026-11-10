import { createFileRoute } from '@tanstack/react-router'
import { RunSetupPage } from '@/features/run-setup/run-setup-page'

export const Route = createFileRoute('/_shell/r/$repo/run-setup')({ component: RunSetup })

function RunSetup() {
  return <RunSetupPage repo={Route.useParams().repo} />
}
