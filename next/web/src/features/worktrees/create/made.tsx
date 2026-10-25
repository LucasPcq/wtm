import { CircleCheck, CircleX } from 'lucide-react'
import type { Applied } from '@/api/queries/commands'
import { CommandLine } from '@/features/worktrees/shared/command-line'
import { CopyPath } from '@/features/worktrees/shared/path'
import { sentence } from '@/features/worktrees/shared/sentence'

export function Made({ envelope }: { envelope: Applied<'create'> }) {
  const { items, follow_ups: followUps } = envelope

  const nextOf = (subject: string) =>
    followUps.find((followUp) =>
      followUp.answers?.some((answer) => answer.path === 'worktree' && answer.values[0] === subject),
    )
  const loose = followUps.filter((followUp) => !items.some((item) => nextOf(item.subject) === followUp))

  return (
    <div className="flex flex-col gap-3 not-first:border-t not-first:pt-4">
      {items.map(({ subject, status, detail }) => {
        const next = nextOf(subject)
        if (status !== 'done' || !detail)
          return (
            <p key={subject} className="flex items-center gap-2 text-sm">
              <CircleX className="size-4 shrink-0 text-destructive" aria-hidden="true" />
              <span className="font-medium">{subject}</span>
              <span className="text-muted-foreground">{status}</span>
            </p>
          )

        return (
          <div key={subject} className="flex items-center gap-3">
            <CircleCheck className="size-4 shrink-0 text-green-500" aria-hidden="true" />
            <div className="flex min-w-0 flex-1 flex-col">
              <span className="text-sm font-medium">{subject}</span>
              <CopyPath path={detail.path} />
            </div>
            {next && <CommandLine followUp={next} inline />}
          </div>
        )
      })}

      {loose.map((followUp) => (
        <div key={followUp.code + JSON.stringify(followUp.answers)} className="flex items-center gap-3 text-sm">
          <span className="flex-1">{sentence(followUp.message)}</span>
          <CommandLine followUp={followUp} inline />
        </div>
      ))}
    </div>
  )
}
