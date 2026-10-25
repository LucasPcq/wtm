import type { QueryClient } from '@tanstack/react-query'
import { Outlet, createRootRouteWithContext } from '@tanstack/react-router'
import { TokenGate } from '@/features/shell/token-gate'

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: () => (
    <TokenGate>
      <Outlet />
    </TokenGate>
  ),
})
