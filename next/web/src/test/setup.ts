import { cleanup } from '@testing-library/react'
import { afterAll, afterEach, beforeAll } from 'vitest'
import { useRemoveDialog } from '@/features/worktrees/hooks/use-remove-dialog'
import { useRuns } from '@/features/worktrees/hooks/use-run'
import { useSelections } from '@/features/worktrees/hooks/use-selection'
import { useUi } from '@/features/worktrees/store/ui-store'
import { server } from '@/test/server'

beforeAll(() => server.listen({ onUnhandledFrame: 'error' }))
afterEach(() => {
  cleanup()
  server.resetHandlers()
  localStorage.clear()
  // the stores outlive a page, as they do in the browser: each test opens a fresh one
  useUi.setState(useUi.getInitialState(), true)
  useRuns.setState(useRuns.getInitialState(), true)
  useRemoveDialog.setState(useRemoveDialog.getInitialState(), true)
  useSelections.setState(useSelections.getInitialState(), true)
})
afterAll(() => server.close())
