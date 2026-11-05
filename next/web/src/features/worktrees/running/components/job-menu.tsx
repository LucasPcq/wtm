import { Copy, Download, Ellipsis, ExternalLink, FileText } from 'lucide-react'
import { toast } from 'sonner'
import { downloadLog, type JobAt } from '@/api/queries/job-logs'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { copy } from '@/features/worktrees/utils/copy'

export function JobMenu({ at, url, log }: { at: JobAt; url: string | undefined; log: string | undefined }) {
  const save = () => downloadLog(at).catch((failed: Error) => toast.error(failed.message))

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={`More for ${at.job}`}>
          <Ellipsis />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
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
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
