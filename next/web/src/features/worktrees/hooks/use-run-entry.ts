import { useQueryClient } from '@tanstack/react-query'
import { commandLineQuery, shellLine } from '@/api/queries/command-line'
import { narrowed, readQuery } from '@/api/queries/commands'
import type { DomainFollowUp, ListDetail, TreeDetail } from '@/api/types.gen'
import { noFetch, unitsOf } from '@/features/worktrees/fast-forward/utils/plan'
import { toldOf } from '@/features/worktrees/fast-forward/utils/recap'
import { useDirectRun } from '@/features/worktrees/hooks/use-direct-run'
import { openExec } from '@/features/worktrees/hooks/use-exec-dialog'
import { openFastForward, useFastForwardDialog } from '@/features/worktrees/hooks/use-fast-forward-dialog'
import { openRemove } from '@/features/worktrees/hooks/use-remove-dialog'
import { openReparent } from '@/features/worktrees/hooks/use-reparent-dialog'
import { useReparentNow } from '@/features/worktrees/hooks/use-reparent-now'
import { type Entry, leftOf, menu, type Subject, subjectsOf } from '@/features/worktrees/utils/actions'
import { copy } from '@/features/worktrees/utils/copy'

export type RunEntry = ReturnType<typeof useRunEntry>

export function useRunEntry(repo: string) {
  const client = useQueryClient()
  const direct = useDirectRun(repo, 'fast-forward')
  const reparentNow = useReparentNow(repo)

  // the copy starts in the click; its line is spelled by the server meanwhile
  const copyLine = (entry: Extract<Entry, { followUp: unknown }>) =>
    copy(
      client.fetchQuery(commandLineQuery(entry.followUp)).then(({ argv }) => {
        if (argv.length === 0) throw new Error(`No command line for this: ${entry.followUp.message}`)
        return shellLine(argv)
      }),
    )

  // what the reads on screen say of a worktree's changes: a dirty one is never asked unforced
  const dirty = (worktree: string) =>
    (['list', 'tree'] as const).some((command) => {
      const read = client.getQueryData(readQuery(repo, command).queryKey)
      return (
        read &&
        narrowed<ListDetail | TreeDetail>(read).items.some((item) => item.subject === worktree && item.detail?.dirty)
      )
    })

  // one worktree runs at once, unless its plan has something to decide, or the reads show it dirty; several open the
  // dialog on Confirm
  const fastForward = (followUp: DomainFollowUp, rows: Subject[]) => {
    const worktrees = subjectsOf(followUp)
    const [only] = worktrees
    const dialog = () => openFastForward(repo, { worktrees, left: leftOf(rows, 'fast_forward'), picking: false })
    if (worktrees.length !== 1 || !only || dirty(only)) return dialog()
    const answers = followUp.answers ?? []
    void direct({
      subjects: worktrees,
      answers,
      applying: [...answers, noFetch],
      unitsOf,
      doing: `Fast-forwarding ${only}`,
      told: (envelope) => toldOf(only, envelope),
      shown: () => useFastForwardDialog.getState().open,
      onDecide: dialog,
      onDetails: (held) => openFastForward(repo, { worktrees: held, left: [], picking: false }),
    })
  }

  // a move the core spelled, its new parent given (a callout's fix), runs at once; one worktree opens on its new
  // parent, a selection on its worktrees
  const reparent = (followUp: DomainFollowUp, rows: Subject[]) => {
    if (followUp.answers?.some((answer) => answer.path === 'to')) return void reparentNow([followUp])
    openReparent(repo, { worktrees: subjectsOf(followUp), left: leftOf(rows, 'reparent'), picking: rows.length > 1 })
  }

  return (entry: Entry, rows: Subject[]) => {
    if (!('followUp' in entry)) return

    switch (menu[entry.action].runs) {
      case 'copy':
        return copyLine(entry)
      case 'link':
        return void (entry.followUp.url && window.open(entry.followUp.url, '_blank', 'noreferrer'))
      case 'web':
        if (entry.action === 'fast_forward') return fastForward(entry.followUp, rows)
        if (entry.action === 'reparent') return reparent(entry.followUp, rows)
        if (entry.action === 'exec')
          return openExec(repo, { worktrees: subjectsOf(entry.followUp), left: leftOf(rows, entry.action) })
        return openRemove(repo, {
          worktrees: subjectsOf(entry.followUp),
          left: leftOf(rows, entry.action),
          prechecked: true,
        })
    }
  }
}
