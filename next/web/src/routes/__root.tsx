import type { QueryClient } from '@tanstack/react-query'
import { Outlet, createRootRouteWithContext } from '@tanstack/react-router'
import { TooltipProvider } from '@/components/ui/tooltip'
import { TokenGate } from '@/features/shell/token-gate'

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: () => (
    <TooltipProvider>
      <TokenGate>
        <Outlet />
      </TokenGate>
    </TooltipProvider>
  ),
})
