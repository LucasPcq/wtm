import { type Asked, createWords, skipWords } from '@/features/worktrees/create/utils/words'

export function NotAsked({ fields }: { fields: Asked[] }) {
  const said = fields.flatMap((field) => {
    const reason = field.skip && skipWords[field.skip]
    return reason ? [{ path: field.path, reason }] : []
  })

  if (said.length === 0) return null

  return (
    <ul className="border-t pt-3 text-xs text-muted-foreground">
      {said.map(({ path, reason }) => (
        <li key={path}>
          Not asked: {createWords[path].label}, {reason}.
        </li>
      ))}
    </ul>
  )
}
