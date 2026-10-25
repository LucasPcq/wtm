import { cleanup } from '@testing-library/react'
import { afterAll, afterEach, beforeAll } from 'vitest'
import { useCreateRuns } from '@/features/worktrees/create/hooks/use-create-run'
import { useRemoveDialog } from '@/features/worktrees/hooks/use-remove-dialog'
import { useRemoveRuns } from '@/features/worktrees/remove/hooks/use-remove-run'
import { useUi } from '@/features/worktrees/store/ui-store'
import { server } from '@/test/server'

beforeAll(() => server.listen({ onUnhandledFrame: 'error' }))
afterEach(() => {
  cleanup()
  server.resetHandlers()
  localStorage.clear()
  // the stores outlive a page, as they do in the browser: each test opens a fresh one
  useUi.setState(useUi.getInitialState(), true)
  useCreateRuns.setState(useCreateRuns.getInitialState(), true)
  useRemoveDialog.setState(useRemoveDialog.getInitialState(), true)
  useRemoveRuns.setState(useRemoveRuns.getInitialState(), true)
})
afterAll(() => server.close())
