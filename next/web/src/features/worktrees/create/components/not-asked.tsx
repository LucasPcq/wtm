import type { DomainField } from '@/api/types.gen'
import { skipWords, wordsOf } from '@/features/worktrees/create/utils/words'

export function NotAsked({ fields }: { fields: DomainField[] }) {
  const said = fields.flatMap((field) => {
    const reason = field.skip && skipWords[field.skip]
    return reason ? [{ path: field.path, reason }] : []
  })

  if (said.length === 0) return null

  return (
    <ul className="border-t pt-3 text-xs text-muted-foreground">
      {said.map(({ path, reason }) => (
        <li key={path}>
          Not asked: {wordsOf(path).label}, {reason}.
        </li>
      ))}
    </ul>
  )
}
