import { toast } from 'sonner'

export function copy(line: string) {
  navigator.clipboard.writeText(line).then(
    () => toast.success('Copied', { description: line }),
    () => toast.error("Couldn't copy: the browser refused the clipboard", { description: line }),
  )
}
