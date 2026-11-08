import { createFileRoute, Outlet, useParams } from '@tanstack/react-router'
import { Shell } from '@/features/shell/shell'
import { DaemonPanel } from '@/features/worktrees/daemon/daemon-panel'

export const Route = createFileRoute('/_shell')({ component: Layout })

function Layout() {
  const repo = useParams({ strict: false }).repo

  return (
    <Shell repo={repo} daemon={repo ? (trigger) => <DaemonPanel repo={repo}>{trigger}</DaemonPanel> : undefined}>
      <Outlet />
    </Shell>
  )
}
