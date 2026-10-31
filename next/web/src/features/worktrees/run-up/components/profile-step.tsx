import { Search } from 'lucide-react'
import { useId, useState } from 'react'
import type { DomainOption } from '@/api/types.gen'
import { ChoiceOptions } from '@/components/choice-options'
import { InputGroup, InputGroupAddon, InputGroupInput } from '@/components/ui/input-group'
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
      <InputGroup className="rounded-none border-0 border-b shadow-none has-[[data-slot=input-group-control]:focus-visible]:ring-0">
        <InputGroupAddon>
          <Search />
        </InputGroupAddon>
        <InputGroupInput
          aria-label="Search the profiles"
          placeholder="Search profiles"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
      </InputGroup>
      <ChoiceOptions
        id={`${id}-profile`}
        label="Profile"
        value={value}
        options={visible.map((option) => ({
          value: option.value,
          title: option.value,
          description: profileLine(option),
        }))}
        onChange={onChange}
      />
    </section>
  )
}
