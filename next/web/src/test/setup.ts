import { cleanup } from '@testing-library/react'
import { afterAll, afterEach, beforeAll } from 'vitest'
import { useCreateRuns } from '@/features/worktrees/hooks/use-create-run'
import { useUi } from '@/features/worktrees/ui-store'
import { server } from '@/test/server'

beforeAll(() => server.listen({ onUnhandledFrame: 'error' }))
afterEach(() => {
  cleanup()
  server.resetHandlers()
  localStorage.clear()
  // the stores outlive a page, as they do in the browser: each test opens a fresh one
  useUi.setState(useUi.getInitialState(), true)
  useCreateRuns.setState(useCreateRuns.getInitialState(), true)
})
afterAll(() => server.close())
