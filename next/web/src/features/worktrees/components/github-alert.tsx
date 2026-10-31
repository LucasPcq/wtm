import type { ReactNode } from 'react'
import { Alert } from '@/features/worktrees/components/alert'
import { CopyCommand } from '@/features/worktrees/components/copy-command'
import { type GitHubFact, githubWords } from '@/features/worktrees/utils/github'

// every GitHub that could not be asked: its reason's words, its fix on the title line unless the screen puts its own
// way on there or is too narrow for it (then the fix goes below, full width), and what the screen alone adds to say
export function GitHubAlert({
  fact,
  also,
  action,
  fixBelow = false,
}: {
  fact: GitHubFact
  also?: string
  action?: ReactNode
  fixBelow?: boolean
}) {
  const { title, description, command } = githubWords(fact)

  return (
    <Alert
      tone="amber"
      title={title}
      description={also ? `${description} ${also}` : description}
      action={action ?? (!fixBelow && command && <CopyCommand text={command} look="inline" />)}
      below={(action || fixBelow) && command && <CopyCommand text={command} />}
    />
  )
}
