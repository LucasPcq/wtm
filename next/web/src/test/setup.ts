import { cleanup } from '@testing-library/react'
import { toast } from 'sonner'
import { afterAll, afterEach, beforeAll } from 'vitest'
import { useExecDialog } from '@/features/worktrees/hooks/use-exec-dialog'
import { useFastForwardDialog } from '@/features/worktrees/hooks/use-fast-forward-dialog'
import { usePruneDialog } from '@/features/worktrees/hooks/use-prune-dialog'
import { useRemoveDialog } from '@/features/worktrees/hooks/use-remove-dialog'
import { useReparentDialog } from '@/features/worktrees/hooks/use-reparent-dialog'
import { useRuns } from '@/features/worktrees/hooks/use-run'
import { useRunUpDialogs } from '@/features/worktrees/hooks/use-run-up-dialog'
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
  usePruneDialog.setState(usePruneDialog.getInitialState(), true)
  useFastForwardDialog.setState(useFastForwardDialog.getInitialState(), true)
  useExecDialog.setState(useExecDialog.getInitialState(), true)
  useReparentDialog.setState(useReparentDialog.getInitialState(), true)
  useRunUpDialogs.setState(useRunUpDialogs.getInitialState(), true)
  useSelections.setState(useSelections.getInitialState(), true)
  // Sonner replays every toast still active to the next Toaster mounted: the next test's page
  toast.dismiss()
})
afterAll(() => server.close())
