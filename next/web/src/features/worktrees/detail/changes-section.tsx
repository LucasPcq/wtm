import type { StatusDetail } from '@/api/types.gen'
import { Section } from '@/features/worktrees/detail/section'

const listed = 8

export function ChangesSection({ detail: { changes, origin, base } }: { detail: StatusDetail }) {
  const standing = () => {
    if (!origin) return 'not on origin'
    if (origin.ahead === 0 && origin.behind === 0) return 'in step with origin'
    return `origin ↑${origin.ahead} ↓${origin.behind}`
  }

  const files = changes?.files ?? []
  const summary = () => {
    if (!changes || files.length === 0) return 'Working tree clean'
    const { staged, modified, untracked, insertions, deletions } = changes
    return `${staged} staged · ${modified} modified · ${untracked} untracked · +${insertions} −${deletions}`
  }

  return (
    <Section id="changes" meta={base ? `${base.ahead} ahead of ${base.name}` : ''}>
      {changes && (
        <div className="flex flex-col gap-2 text-sm">
          <p className="text-muted-foreground">
            {summary()} · {standing()}
          </p>
          {files.length > 0 && (
            <ul className="flex flex-col gap-0.5 font-mono text-xs">
              {files.slice(0, listed).map(({ path, status }) => (
                <li key={path} className="flex gap-2">
                  <span className="w-20 shrink-0 text-muted-foreground">{status}</span>
                  <span className="truncate">{path}</span>
                </li>
              ))}
              {files.length > listed && <li className="text-muted-foreground">and {files.length - listed} more</li>}
            </ul>
          )}
        </div>
      )}
    </Section>
  )
}
