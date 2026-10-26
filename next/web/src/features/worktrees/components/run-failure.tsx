import { CircleX } from 'lucide-react'

export function RunFailure({ message }: { message: string | undefined }) {
  if (!message) return null

  return (
    <p className="flex items-start gap-3 text-sm">
      <CircleX className="mt-0.5 size-4 shrink-0 text-destructive" aria-hidden="true" />
      {message}
    </p>
  )
}
