import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createMemoryHistory, createRouter, RouterProvider } from '@tanstack/react-router'
import { render } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useUi } from '@/features/worktrees/store/ui-store'
import { routeTree } from '@/routeTree.gen'

// the route's code is loaded before the page renders, so that the first test's wait does not also pay the import
// (past a second on a slow CI runner); the store rehydrates from storage as a page opening does
export async function renderApp(path: string) {
  localStorage.setItem('wtm.token', 't')
  await useUi.persist.rehydrate()
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const router = createRouter({
    routeTree,
    context: { queryClient },
    history: createMemoryHistory({ initialEntries: [path] }),
  })
  await router.load()
  const user = userEvent.setup()
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return { user, router }
}
