import { CircleX } from 'lucide-react'
import { useId } from 'react'
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

export function CommandStep({
  command,
  worktrees,
  problem,
  onType,
  onSubmit,
}: {
  command: string
  worktrees: string[]
  problem: string | undefined
  onType: (command: string) => void
  onSubmit: () => void
}) {
  const id = useId()
  const described = `${id}-description`
  const refused = `${id}-problem`
  const where = worktrees.length === 1 ? worktrees[0] : `each of the ${worktrees.length} worktrees`

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault()
        onSubmit()
      }}
    >
      <Field className="gap-2">
        <div className="flex flex-col gap-1">
          <FieldLabel htmlFor={id}>Command</FieldLabel>
          <FieldDescription id={described}>Run by sh in {where}.</FieldDescription>
        </div>
        <Input
          id={id}
          value={command}
          placeholder="pnpm test"
          autoFocus
          spellCheck={false}
          autoComplete="off"
          aria-invalid={problem !== undefined}
          aria-describedby={problem ? `${described} ${refused}` : described}
          className="font-mono"
          onChange={(event) => onType(event.target.value)}
        />
        {problem && (
          <p id={refused} role="alert" className="flex items-center gap-2 text-xs">
            <CircleX className="size-3.5 shrink-0 text-destructive" aria-hidden="true" />
            {problem}
          </p>
        )}
      </Field>
    </form>
  )
}
