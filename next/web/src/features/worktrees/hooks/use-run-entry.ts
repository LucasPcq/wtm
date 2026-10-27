import { useQueryClient } from '@tanstack/react-query'
import { commandLineQuery, shellLine } from '@/api/queries/command-line'
import { openRemove } from '@/features/worktrees/hooks/use-remove-dialog'
import { type Entry, leftOf, menu, type Subject, subjectsOf } from '@/features/worktrees/utils/actions'
import { copy } from '@/features/worktrees/utils/copy'

export function useRunEntry(repo: string) {
  const client = useQueryClient()

  // the copy starts in the click; its line is spelled by the server meanwhile
  const copyLine = (entry: Extract<Entry, { followUp: unknown }>) =>
    copy(
      client.fetchQuery(commandLineQuery(entry.followUp)).then(({ argv }) => {
        if (argv.length === 0) throw new Error(`No command line for this: ${entry.followUp.message}`)
        return shellLine(argv)
      }),
    )

  return (entry: Entry, rows: Subject[]) => {
    if (!('followUp' in entry)) return

    switch (menu[entry.action].runs) {
      case 'copy':
        return copyLine(entry)
      case 'link':
        return void (entry.followUp.url && window.open(entry.followUp.url, '_blank', 'noreferrer'))
      case 'web':
        return openRemove(repo, {
          worktrees: subjectsOf(entry.followUp),
          left: leftOf(rows, entry.action),
          prechecked: true,
        })
      case 'later':
        return
    }
  }
}
