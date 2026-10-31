import { Info } from 'lucide-react'
import type { DomainWarning } from '@/api/types.gen'
import { CopyCommand } from '@/features/worktrees/components/copy-command'
import { githubWords, isUnavailable } from '@/features/worktrees/utils/github'
import { sentence } from '@/features/worktrees/utils/sentence'

// GitHub that could not be asked says it in GitHub's words, its fix to copy beside the line
const lineOf = (warning: DomainWarning) => {
  if (!isUnavailable(warning)) return { text: sentence(warning.message), command: undefined }
  const { title, description, command } = githubWords(warning)
  return { text: `${title}. ${description}`, command }
}

export function Notes({ warnings }: { warnings: DomainWarning[] }) {
  if (warnings.length === 0) return null

  return (
    <ul className="flex shrink-0 flex-col gap-1 border-t px-8 py-3 text-xs text-muted-foreground">
      {warnings.map((warning) => {
        const { text, command } = lineOf(warning)
        return (
          <li key={warning.code} className="flex items-center gap-1.5">
            <Info className="size-3.5 shrink-0" aria-hidden="true" />
            {text}
            {command && <CopyCommand text={command} look="inline" />}
          </li>
        )
      })}
    </ul>
  )
}
