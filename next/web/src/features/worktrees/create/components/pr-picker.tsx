import { useStore } from '@tanstack/react-form'
import { useDebouncer } from '@tanstack/react-pacer'
import { CircleX, GitPullRequest, Search } from 'lucide-react'
import { useState } from 'react'
import type { DomainFieldError, DomainOption } from '@/api/types.gen'
import { keyOf } from '@/components/command-form/answers'
import type { CommandForm } from '@/components/command-form/use-command-form'
import { Button } from '@/components/ui/button'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { InputGroup, InputGroupAddon, InputGroupInput } from '@/components/ui/input-group'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { CopyCommand } from '@/features/worktrees/create/components/copy-command'
import {
  checkoutWords,
  optionLine,
  type PrFilter,
  prFilterLabel,
  prFilters,
  refusalLine,
} from '@/features/worktrees/create/utils/checkout-words'
import { cn } from '@/lib/utils'

const pr = keyOf('pr')
const mine = keyOf('mine')
const review = keyOf('review')

export function PrPicker({
  form,
  options,
  chosen,
  errors,
  onAsk,
  onNewBranch,
}: {
  form: CommandForm
  options: DomainOption[]
  chosen: string | undefined
  errors: DomainFieldError[]
  onAsk: () => void
  onNewBranch: () => void
}) {
  const [query, setQuery] = useState('')
  const filter = useStore(form.store, (state): PrFilter => {
    if (state.values[mine]?.[0] === 'true') return 'mine'
    if (state.values[review]?.[0] === 'true') return 'review'
    return 'all'
  })
  const { label, help } = checkoutWords(undefined).pr

  const typed = query.trim()
  const byNumber = /^\d+$/.test(typed)
  const matches = (option: DomainOption) => {
    const of = option.pull_request
    if (!of || !typed) return true
    if (byNumber) return String(of.number).startsWith(typed)
    return (of.title ?? '').toLowerCase().includes(typed.toLowerCase())
  }
  const shown = options.filter(matches)
  const base = options[0]?.pull_request?.url.replace(/^https?:\/\//, '').replace(/\/pull\/\d+$/, '')

  const choose = (value: string) => {
    form.setFieldValue(pr, [value])
    onAsk()
  }
  const askNumber = useDebouncer(choose, { wait: 300 })

  // a number no listed pull request starts with is asked of GitHub, which finds a closed or merged one too
  const search = (text: string) => {
    setQuery(text)
    const number = text.trim()
    if (/^\d+$/.test(number) && !options.some((option) => String(option.pull_request?.number).startsWith(number)))
      return askNumber.maybeExecute(number)
    askNumber.cancel()
  }

  const pick = (to: PrFilter) => {
    form.setFieldValue(mine, [String(to === 'mine')])
    form.setFieldValue(review, [String(to === 'review')])
    onAsk()
  }

  const empty = options.length === 0 && !typed && filter === 'all'

  return (
    <section className="flex flex-col gap-2" aria-label={label}>
      <div className="flex flex-col gap-1">
        <h3 className="text-sm font-medium">{label}</h3>
        <p className="text-xs text-pretty text-muted-foreground">
          {base ? `Open pull requests on ${base}; ${help}` : `Open pull requests; ${help}`}
        </p>
      </div>

      <div className="flex flex-col rounded-lg border">
        <InputGroup className="rounded-b-none border-0 border-b shadow-none">
          <InputGroupAddon>
            <Search />
          </InputGroupAddon>
          <InputGroupInput
            aria-label="Search pull requests"
            placeholder="Search by title or number"
            value={query}
            onChange={(e) => search(e.target.value)}
          />
        </InputGroup>

        {!empty && !typed && (
          <ToggleGroup
            type="single"
            variant="outline"
            size="sm"
            aria-label="Pull requests"
            value={filter}
            onValueChange={(value: string) => {
              const next = prFilters.find((known) => known === value)
              if (next) pick(next)
            }}
            className="px-3 pt-3"
          >
            {prFilters.map((known) => (
              <ToggleGroupItem key={known} value={known} className="rounded-full px-3">
                {prFilterLabel[known]}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        )}

        {empty && (
          <Empty className="p-6">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <GitPullRequest />
              </EmptyMedia>
              <EmptyTitle>No open pull request</EmptyTitle>
              <EmptyDescription>
                {base ?? 'The repository'} has none open. A number finds a closed or merged one.
              </EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <Button variant="outline" size="sm" onClick={onNewBranch}>
                New branch
              </Button>
            </EmptyContent>
          </Empty>
        )}

        {!empty && shown.length === 0 && (
          <p className="px-4 py-3 text-sm text-muted-foreground">No pull request matches.</p>
        )}

        {shown.length > 0 && (
          <ul className="flex flex-col p-1.5">
            {shown.map((option) => {
              const of = option.pull_request
              if (!of) return null
              const refused = option.problem !== undefined
              const command = option.problem?.params?.command
              const followUp = option.problem?.follow_up

              return (
                <li key={option.value} className="flex items-center gap-2">
                  <button
                    type="button"
                    aria-pressed={option.value === chosen}
                    disabled={refused}
                    onClick={() => choose(option.value)}
                    className={cn(
                      'flex min-w-0 flex-1 flex-col items-start gap-0.5 rounded-md px-2.5 py-2 text-left outline-none',
                      'hover:bg-muted focus-visible:ring-[3px] focus-visible:ring-ring/50 aria-pressed:bg-muted',
                      'disabled:pointer-events-none disabled:text-muted-foreground',
                    )}
                  >
                    <span className="w-full truncate text-sm">
                      #{of.number} {of.title}
                    </span>
                    <span className="w-full truncate text-xs text-muted-foreground">{optionLine(option)}</span>
                  </button>
                  {command && <CopyCommand text={command} look="copy" />}
                  {!command && followUp && <CommandLine followUp={followUp} look="copy" />}
                </li>
              )
            })}
          </ul>
        )}
      </div>

      {errors.map((error) => (
        <p key={error.code} role="alert" className="flex items-start gap-2 text-sm text-destructive">
          <CircleX className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
          {refusalLine(error)}
        </p>
      ))}
    </section>
  )
}
