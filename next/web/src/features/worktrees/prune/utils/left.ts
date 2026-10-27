import type { Left } from '@/features/worktrees/components/left-item'

// what the person asked for that the run no longer holds: with the core's why when a plan refused it, else as gone from
// the candidates, which the rows read again on every event
export function leftOut({
  dropped,
  asked,
  candidates,
  going,
}: {
  dropped: Left[]
  asked: string[]
  candidates: string[]
  going: string[]
}): Left[] {
  const said = dropped.filter(({ worktree }) => !going.includes(worktree))
  const gone = asked.filter(
    (worktree) => !candidates.includes(worktree) && !said.some((one) => one.worktree === worktree),
  )
  return [...said, ...gone.map((worktree) => ({ worktree, why: 'No longer finished' }))]
}
