import { BookmarkCheck } from 'lucide-react'
import type { ConfigshowRemembered } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { rememberedWords } from '@/features/worktrees/settings/utils/words'

export function RememberedAnswers({
  remembered,
  disabled,
  onForget,
}: {
  remembered: ConfigshowRemembered[]
  disabled: boolean
  onForget: (answer: ConfigshowRemembered) => void
}) {
  if (remembered.length === 0)
    return <p className="text-sm text-muted-foreground">None: Create asks each of its questions.</p>

  return (
    <div className="flex flex-col gap-4">
      {remembered.map((answer) => {
        const words = rememberedWords[answer.question]
        const said = `${words?.question ?? answer.question}: ${words?.values[answer.value] ?? answer.value}`

        return (
          <div key={answer.entry} className="flex items-start gap-3 rounded-lg border px-4 py-3">
            <BookmarkCheck className="mt-0.5 size-4 text-muted-foreground" aria-hidden="true" />
            <div className="flex min-w-0 flex-col gap-1">
              <span className="text-sm font-medium">{said}</span>
              <span className="text-sm text-muted-foreground">
                Create doesn't ask it. Forget it and Create asks again.
              </span>
            </div>
            <Button
              variant="outline"
              size="sm"
              className="ml-auto"
              disabled={disabled}
              aria-label={`Forget ${said}`}
              onClick={() => onForget(answer)}
            >
              Forget
            </Button>
          </div>
        )
      })}
    </div>
  )
}
