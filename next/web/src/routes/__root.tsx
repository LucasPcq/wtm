import type { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router'
import { TokenGate } from '@/features/shell/components/token-gate'

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: () => (
    <TokenGate>
      <Outlet />
    </TokenGate>
  ),
})
