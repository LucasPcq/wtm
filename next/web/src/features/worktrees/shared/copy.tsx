import { Check } from 'lucide-react'
import { toast } from 'sonner'

export function copy(line: string) {
  navigator.clipboard.writeText(line).then(
    () => toast.success('Copied', { description: line, icon: <Check className="size-4" /> }),
    () => toast.error("Couldn't copy: the browser refused the clipboard", { description: line }),
  )
}
