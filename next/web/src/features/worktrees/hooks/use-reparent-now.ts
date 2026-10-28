import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { commandLineQuery, shellLine } from '@/api/queries/command-line'
import type { Applied } from '@/api/queries/commands'
import type { DomainFollowUp } from '@/api/types.gen'
import { useDirectRun } from '@/features/worktrees/hooks/use-direct-run'
import { openReparent, useReparentDialog } from '@/features/worktrees/hooks/use-reparent-dialog'
import { succeeded } from '@/features/worktrees/hooks/use-run'
import { byParent, syncOf, undosOf, unitsOf } from '@/features/worktrees/reparent/utils/plan'
import { refusalOf, toldOf } from '@/features/worktrees/reparent/utils/words'
import { copy } from '@/features/worktrees/utils/copy'
import { listed } from '@/features/worktrees/utils/listed'
import { requestOf } from '@/features/worktrees/utils/reparent'

// a move the core spelled (a drop, a fix, a child left orphaned, an undo) runs at once: one run per new parent, one
// after the other, and one toast for them all, with the undo of each and the sync of all
export function useReparentNow(repo: string) {
  const client = useQueryClient()
  const direct = useDirectRun(repo, 'reparent')

  const copyLine = (followUp: DomainFollowUp) =>
    copy(
      client.fetchQuery(commandLineQuery(followUp)).then(({ argv }) => {
        if (argv.length === 0) throw new Error(`No command line for this: ${followUp.message}`)
        return shellLine(argv)
      }),
    )

  const now = async (followUps: DomainFollowUp[]) => {
    const runs = byParent(followUps)
    const ran: Applied<'reparent'>[] = []

    for (const [i, { to, worktrees }] of runs.entries()) {
      const answers = requestOf({ worktrees, to })
      const last = i === runs.length - 1
      const envelope = await direct({
        subjects: worktrees,
        answers,
        applying: answers,
        unitsOf,
        doing: `Reparenting ${listed(worktrees)}`,
        told: (envelope, refused) => {
          const all = envelope ? [...ran, envelope] : ran
          const undos = undosOf(all)
          const sync = syncOf(all)
          return {
            ...toldOf(envelope ? all : [], refused),
            ...(!envelope && { title: `Couldn't reparent ${listed(worktrees)}` }),
            // Sonner draws cancel before action: Undo comes first
            cancel: undos.length > 0 ? { label: 'Undo', onClick: () => void now(undos) } : undefined,
            action: sync ? { label: 'Copy wtm sync', onClick: () => copyLine(sync) } : undefined,
          }
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
      if (!envelope) return
      ran.push(envelope)
      if (!succeeded(envelope)) return
    }
  }

  return now
}
