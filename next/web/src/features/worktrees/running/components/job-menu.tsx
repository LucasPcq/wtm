import { Copy, Download, Ellipsis, ExternalLink, FileText } from 'lucide-react'
import { toast } from 'sonner'
import { downloadLog, type JobAt } from '@/api/queries/job-logs'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { MenuEntry } from '@/features/worktrees/components/menu-entry'
import type { JobActs } from '@/features/worktrees/running/hooks/use-job-actions'
import { actEntry, actLook } from '@/features/worktrees/running/utils/job-words'
import { copy } from '@/features/worktrees/utils/copy'

export function JobMenu({
  at,
  url,
  log,
  acts,
}: {
  at: JobAt
  url: string | undefined
  log: string | undefined
  acts: JobActs['acts']
}) {
  const save = () => downloadLog(at).catch((failed: Error) => toast.error(failed.message))

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={`More for ${at.job}`}>
          <Ellipsis />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        <DropdownMenuItem disabled={!url} onSelect={() => url && window.open(url, '_blank', 'noreferrer')}>
          <ExternalLink />
          Open the address
        </DropdownMenuItem>
        <DropdownMenuItem disabled={!url} onSelect={() => url && copy(url)}>
          <Copy />
          Copy the address
        </DropdownMenuItem>
        <DropdownMenuItem disabled={!log} onSelect={() => log && copy(log)}>
          <FileText />
          Copy the log file’s path
        </DropdownMenuItem>
        <DropdownMenuItem disabled={!log} onSelect={() => void save()}>
          <Download />
          Download the log
        </DropdownMenuItem>
        {acts.length > 0 && <DropdownMenuSeparator />}
        {acts.map(({ act, why, keptBy, run }) => (
          <MenuEntry
            key={act}
            Item={DropdownMenuItem}
            icon={actLook[act].icon}
            {...actEntry(act, at, keptBy)}
            why={why}
            onSelect={run}
          />
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
