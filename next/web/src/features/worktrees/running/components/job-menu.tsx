import { Copy, Download, Ellipsis, ExternalLink, FileText, Pin, PinOff } from 'lucide-react'
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
  shared,
  holders,
  pinned,
  onPin,
}: {
  at: JobAt
  url: string | undefined
  log: string | undefined
  acts: JobActs['acts']
  shared: boolean
  holders: number
  pinned: boolean
  onPin: () => void
}) {
  const save = () => downloadLog(at).catch((failed: Error) => toast.error(failed.message))

  const entries = acts.map(({ act, why, keptBy, run }) => (
    <MenuEntry
      key={act}
      Item={DropdownMenuItem}
      icon={actLook[act].icon}
      {...actEntry(act, { at, keptBy, holders: shared ? holders : undefined })}
      why={why}
      onSelect={run}
    />
  ))

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={`More for ${at.job}`}>
          <Ellipsis />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        {/* a shared service's menu opens on what acts on it, as what it does reaches the others */}
        {shared && entries}
        {shared && <DropdownMenuSeparator />}
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
        {!shared && acts.length > 0 && <DropdownMenuSeparator />}
        {!shared && entries}
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={onPin}>
          {pinned ? <PinOff /> : <Pin />}
          {pinned ? 'Unpin' : 'Pin to the top'}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
