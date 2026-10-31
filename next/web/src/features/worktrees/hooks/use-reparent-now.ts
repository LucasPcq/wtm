import { toast } from 'sonner'
import type { Applied } from '@/api/queries/commands'
import type { DomainFollowUp } from '@/api/types.gen'
import { type Told, useDirectRun } from '@/features/worktrees/hooks/use-direct-run'
import { openReparent, useReparentDialog } from '@/features/worktrees/hooks/use-reparent-dialog'
import { dismissEnded, succeeded } from '@/features/worktrees/hooks/use-run'
import { openSync } from '@/features/worktrees/hooks/use-sync-dialog'
import { byParent, syncOf, undosOf, unitsOf, unreported } from '@/features/worktrees/reparent/utils/plan'
import { refusalOf, toldOf } from '@/features/worktrees/reparent/utils/words'
import { listed } from '@/features/worktrees/utils/listed'
import { requestOf } from '@/features/worktrees/utils/reparent'

// a move the core spelled (a drop, a fix, a child left orphaned, an undo) runs at once: one run per new parent, one
// after the other, and one toast for them all, with the undo of each and the sync of all
export function useReparentNow(repo: string) {
  const direct = useDirectRun(repo, 'reparent')

  // Undo moves back what moved; on a failure it says how many moves it undoes, since the title names what did not move
  const buttons = (ran: Applied<'reparent'>[], success: boolean): Pick<Told, 'action' | 'cancel'> => {
    const undos = undosOf(ran)
    const sync = syncOf(ran)
    const count = `${undos.length} ${undos.length === 1 ? 'move' : 'moves'}`
    return {
      // Sonner draws cancel before action: Undo comes first
      cancel:
        undos.length > 0
          ? {
              label: success ? 'Undo' : `Undo ${count}`,
              // clicked, the toast was read: a failed run it told of no longer holds the undo back
              onClick: () => {
                dismissEnded(repo, 'reparent')
                void now(undos)
              },
            }
          : undefined,
      // sync runs in the web: its dialog opens on the plan of the worktrees moved
      action: sync
        ? {
            label: 'Sync',
            onClick: () => openSync(repo, { answers: sync.answers ?? [], left: [], picking: false, deciding: false }),
          }
        : undefined,
    }
  }

  const now = async (followUps: DomainFollowUp[]) => {
    const runs = byParent(followUps)
    const ran: Applied<'reparent'>[] = []

    for (const [i, { to, worktrees }] of runs.entries()) {
      const answers = requestOf({ worktrees, to })
      const last = i === runs.length - 1
      let told = false
      const envelope = await direct({
        subjects: worktrees,
        answers,
        applying: answers,
        unitsOf,
        doing: `Reparenting ${listed(worktrees)}`,
        told: (envelope, refused) => {
          told = true
          const all = envelope ? [...ran, envelope] : ran
          const said = toldOf(all, { worktrees, refused })
          return { ...said, ...buttons(all, said.tone === 'success') }
        },
        // the runs before the last say nothing of their own when they went well: the last says it of them all
        shown: (envelope) => useReparentDialog.getState().open || (!last && succeeded(envelope)),
        onDecide: (dry) => {
          const ask = { worktrees, left: [], picking: false, to }
          if (dry?.error?.kind !== 'invalid') return openReparent(repo, ask)
          toast.error(`Couldn't reparent ${listed(worktrees)}`, {
            description: refusalOf(dry.error),
            action: { label: 'Details', onClick: () => openReparent(repo, ask) },
          })
        },
        onDetails: (held) => openReparent(repo, { worktrees: held, left: [], picking: false }),
      })
      // a chain that ends here and told nothing (stopped before this run applied, or the dialog open on it) still says
      // what moved, with its Undo
      const goesOn = envelope !== undefined && !last && succeeded(envelope)
      const untold = unreported({ ran, envelope, told, goesOn })
      if (untold.length > 0) {
        const said = toldOf(untold, { worktrees })
        toast[said.tone](said.title, {
          description: said.description,
          ...buttons(untold, said.tone === 'success'),
          duration: 10_000,
        })
      }
      if (!envelope) return
      ran.push(envelope)
      if (!succeeded(envelope)) return
    }
  }

  return now
}
