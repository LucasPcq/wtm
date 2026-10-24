import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createRouter } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { HttpError } from '@/api/fetch'
import { onTokenChange, takeFragment } from '@/api/token'
import { routeTree } from './routeTree.gen'
import './index.css'

const repo = takeFragment()
if (repo && location.pathname === '/') history.replaceState(null, '', `/r/${encodeURIComponent(repo)}/worktrees`)

const queryClient = new QueryClient({
  defaultOptions: {
    // a refusal does not change by asking again; a server gone for a moment may answer
    queries: { retry: (count, error) => !(error instanceof HttpError && error.status < 500) && count < 2, refetchOnWindowFocus: false },
  },
})
// a new token, from a tab a restarted server opened: everything is asked again with it, the event stream included
onTokenChange(() => void queryClient.invalidateQueries())

const router = createRouter({ routeTree, context: { queryClient }, defaultPreload: 'intent' })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
)
