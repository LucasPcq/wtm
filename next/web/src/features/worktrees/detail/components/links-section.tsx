import type { StatusDetail, StatusEnvFile } from '@/api/types.gen'
import { Section } from '@/features/worktrees/detail/components/section'

export function LinksSection({ detail: { parent, children, env } }: { detail: StatusDetail }) {
  const drift = ({ target, missing, keys }: StatusEnvFile) => {
    if (missing) return `${target}: missing`
    const counts = [
      keys.missing && `${keys.missing} missing`,
      keys.conflicting && `${keys.conflicting} differ`,
      keys.orphan && `${keys.orphan} of its own`,
    ]
    return `${target}: ${counts.filter(Boolean).join(', ')}`
  }

  const envLine = () => {
    if (env.declared === 0) return 'none declared'
    const drifted = (env.files ?? []).filter(
      ({ missing, keys }) => missing || keys.missing || keys.conflicting || keys.orphan,
    )
    if (drifted.length > 0) return drifted.map(drift).join('; ')
    return `${env.declared} ${env.declared === 1 ? 'file' : 'files'}, in step`
  }

  return (
    <Section id="links">
      <dl className="grid grid-cols-[6rem_1fr] gap-x-3 gap-y-1.5 text-sm">
        <dt className="text-muted-foreground">Parent</dt>
        <dd>{parent || '—'}</dd>
        <dt className="text-muted-foreground">Children</dt>
        <dd>{children?.length ? children.join(', ') : '—'}</dd>
        <dt className="text-muted-foreground">.env</dt>
        <dd>{envLine()}</dd>
      </dl>
    </Section>
  )
}
