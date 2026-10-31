import type { DomainField } from '@/api/types.gen'
import type { FieldWords } from '@/components/command-form/command-fields'
import { skipWords } from '@/features/worktrees/create/utils/words'

export function NotAsked<P extends string>({
  fields,
  words,
}: {
  fields: (DomainField & { path: P })[]
  words: Record<P, FieldWords>
}) {
  const said = fields.flatMap((field) => {
    const reason = field.skip && skipWords[field.skip]
    return reason ? [{ path: field.path, reason }] : []
  })

  if (said.length === 0) return null

  return (
    <ul className="border-t pt-3 text-xs text-muted-foreground">
      {said.map(({ path, reason }) => (
        <li key={path}>
          Not asked: {words[path].label}, {reason}.
        </li>
      ))}
    </ul>
  )
}
