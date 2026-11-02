import { useId, useState } from 'react'
import type { DomainOption } from '@/api/types.gen'
import { Item, ItemContent, ItemDescription, ItemTitle } from '@/components/ui/item'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { ItemList } from '@/features/worktrees/components/item-list'
import { SearchRow } from '@/features/worktrees/components/search-row'
import { profileLine } from '@/features/worktrees/run-up/utils/words'
import { searched } from '@/features/worktrees/utils/filters'
import { plural } from '@/lib/plural'

export function ProfileStep({
  worktrees,
  options,
  value,
  onChange,
}: {
  worktrees: string[]
  options: DomainOption[]
  value: string
  onChange: (profile: string) => void
}) {
  const id = useId()
  const [query, setQuery] = useState('')
  const visible = options.filter((option) => searched(option.value, query))
  const forWhom = worktrees.length > 2 ? plural(worktrees.length, 'worktree') : worktrees.join(' and ')

  return (
    <section aria-labelledby={id} className="flex flex-col gap-3">
      <div className="flex flex-col gap-1">
        <h3 id={id} className="text-sm font-medium">
          Profile
        </h3>
        <p className="text-xs text-muted-foreground">
          For {forWhom} · {plural(options.length, 'profile')} in run.toml.
        </p>
      </div>
      <RadioGroup aria-label="Profile" value={value} onValueChange={onChange} className="gap-0">
        <ItemList
          label="Profiles"
          header={
            <SearchRow label="Search the profiles" placeholder="Search profiles" query={query} onQuery={setQuery} />
          }
        >
          {visible.map((option) => (
            <Item key={option.value} asChild className="items-start gap-3 rounded-none px-4 py-3">
              <label htmlFor={`${id}-${option.value}`}>
                <RadioGroupItem
                  id={`${id}-${option.value}`}
                  aria-label={option.value}
                  value={option.value}
                  className="mt-0.5"
                />
                <ItemContent className="gap-1">
                  <ItemTitle className="min-w-0 font-normal">{option.value}</ItemTitle>
                  <ItemDescription className="text-xs">{profileLine(option)}</ItemDescription>
                </ItemContent>
              </label>
            </Item>
          ))}
        </ItemList>
      </RadioGroup>
    </section>
  )
}
