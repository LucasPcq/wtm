import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createMemoryHistory, createRouter } from '@tanstack/react-router'
import { render } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { routeTree } from '@/routeTree.gen'

// renderApp renders the whole page at path, as the browser opens it with the token of the server, and the person
// who uses it. The route's code is split and loaded on first use: it is loaded before the page renders, so the
// first test's wait for the screen does not also pay the import of the routes (past a second on a slow CI runner).
export async function renderApp(path: string) {
  localStorage.setItem('wtm.token', 't')
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
