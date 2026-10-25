import { createFileRoute, Outlet, useParams } from '@tanstack/react-router'
import { Shell } from '@/features/shell/shell'

export const Route = createFileRoute('/_shell')({ component: Layout })

function Layout() {
  return (
    <Shell repo={useParams({ strict: false }).repo}>
      <Outlet />
    </Shell>
  )
}
