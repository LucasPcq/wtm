import { Pencil, SquareTerminal } from 'lucide-react'
import type { DomainHooksField } from '@/api/types.gen'
import { spellHook } from '@/components/command-form/answers'
import { hookWords } from '@/features/worktrees/settings/utils/words'

export function HookList({
  field,
  list,
  label,
  onChange,
}: {
  field: DomainHooksField
  list: 'on_create' | 'on_clean'
  label: string
  onChange: () => void
}) {
  const words = hookWords[list]

  return (
    <div className="flex flex-col gap-2">
      <span className="text-sm font-medium">{label}</span>
      <div className="flex flex-col divide-y rounded-lg border">
        {field.entries.map((hook) => (
          <div key={spellHook(hook)} className="flex items-center gap-3 px-4 py-3">
            <SquareTerminal className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <code className="truncate font-mono text-sm">{hook.cmd}</code>
            <span className="shrink-0 text-xs text-muted-foreground">
              {hook.cwd ? `in ${hook.cwd}` : "in the worktree's root"}
            </span>
            <span className="ml-auto shrink-0 text-xs text-muted-foreground">
              {hook.continue_on_error ? words.goesOn : words.stops}
            </span>
          </div>
        ))}
        <button
          type="button"
          onClick={onChange}
          className="flex items-center gap-3 px-4 py-3 text-sm hover:bg-muted/40"
          aria-label={`Change the list: ${label}`}
        >
          <Pencil className="size-4" aria-hidden="true" />
          Change the list
        </button>
      </div>
    </div>
  )
}
