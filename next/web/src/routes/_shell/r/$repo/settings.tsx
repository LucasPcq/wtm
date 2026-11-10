import { createFileRoute } from '@tanstack/react-router'
import { SettingsPage } from '@/features/worktrees/settings/settings-page'

export const Route = createFileRoute('/_shell/r/$repo/settings')({ component: Settings })

function Settings() {
  return <SettingsPage repo={Route.useParams().repo} />
}
