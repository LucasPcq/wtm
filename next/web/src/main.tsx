import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { HttpError } from '@/api/fetch'
import { onTokenChange, takeFragment } from '@/api/token'
import { routeTree } from '@/routeTree.gen'
import './index.css'

const repo = takeFragment()
if (repo && location.pathname === '/') history.replaceState(null, '', `/r/${encodeURIComponent(repo)}/worktrees`)

const queryClient = new QueryClient({
  defaultOptions: {
    // a refusal does not change by asking again; a server gone for a moment may answer
    queries: {
      retry: (count, error) => !(error instanceof HttpError && error.status < 500) && count < 2,
      refetchOnWindowFocus: false,
    },
  },
})
// a new token comes from a restarted server, maybe of another version: its page, not this one's code, reads it
onTokenChange(() => location.reload())
// an address pasted into this tab changes its fragment only: reading it again keeps its token out of the address bar
addEventListener('hashchange', () => location.reload())

const router = createRouter({ routeTree, context: { queryClient }, defaultPreload: 'intent' })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

const root = document.getElementById('root')
if (!root) throw new Error('index.html has no #root to render into')

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
)
