import { Plus, X } from 'lucide-react'
import { useState } from 'react'
import type { DomainField } from '@/api/types.gen'
import type { CommandForm } from '@/components/command-form/use-command-form'
import { Button } from '@/components/ui/button'
import { FieldContent, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Setting } from '@/features/worktrees/set-up/components/setting'
import { fileFound, isStrategy, settingLabel, strategyCard } from '@/features/worktrees/set-up/utils/words'

export function EnvStep({
  form,
  strategy,
  files,
  problems,
}: {
  form: CommandForm
  strategy: Extract<DomainField, { kind: 'select' }>
  files: Extract<DomainField, { kind: 'multi_select' }>
  problems: Record<string, string>
}) {
  const templates = files.options.filter((option) => option.template).length

  return (
    <div className="flex flex-col gap-6">
      <Setting form={form} field={strategy} label={settingLabel.env_strategy} problems={problems}>
        {({ id, values, onChange, invalid }) => (
          <RadioGroup
            id={id}
            aria-label={settingLabel.env_strategy}
            aria-invalid={invalid}
            value={values[0] ?? ''}
            onValueChange={(value) => onChange([value])}
            className="grid grid-cols-1 gap-3 sm:grid-cols-2"
          >
            {strategy.options.map(({ value }) => {
              if (!isStrategy(value)) return null
              const card = strategyCard[value]
              const line = value === 'example' && templates > 0 ? `${card.line} Found ${templates}.` : card.line
              return (
                <FieldLabel
                  key={value}
                  htmlFor={`${id}-${value}`}
                  className="flex w-full items-start gap-3 rounded-lg border p-4 has-data-[state=checked]:border-foreground"
                >
                  <RadioGroupItem id={`${id}-${value}`} value={value} className="mt-0.5" />
                  <FieldContent className="gap-1">
                    <span className="flex items-center gap-2 font-medium">
                      {card.title}
                      {strategy.origin === 'default' && strategy.value === value && (
                        <span className="text-xs font-normal text-muted-foreground">default</span>
                      )}
                    </span>
                    {card.example && <span className="font-mono text-xs">{card.example}</span>}
                    <span className="text-xs font-normal text-muted-foreground">{line}</span>
                  </FieldContent>
                </FieldLabel>
              )
            })}
          </RadioGroup>
        )}
      </Setting>

      {files.skip !== 'no_env' && (
        <Setting form={form} field={files} label={settingLabel.env_files} problems={problems}>
          {({ id, values, onChange }) => <FileList id={id} files={files} values={values} onChange={onChange} />}
        </Setting>
      )}
    </div>
  )
}

function FileList({
  id,
  files,
  values,
  onChange,
}: {
  id: string
  files: Extract<DomainField, { kind: 'multi_select' }>
  values: string[]
  onChange: (values: string[]) => void
}) {
  const [typed, setTyped] = useState<string>()

  const add = () => {
    const target = typed?.trim()
    setTyped(undefined)
    if (target && !values.includes(target)) onChange([...values, target])
  }

  return (
    <ul id={id} className="divide-y rounded-lg border">
      {values.map((target) => (
        <li key={target} className="flex items-center gap-3 px-3 py-2">
          <span className="min-w-0 flex-1 truncate font-mono text-sm" title={target}>
            {target}
          </span>
          <span className="shrink-0 text-xs text-muted-foreground">
            {fileFound(files.options.find((option) => option.value === target)?.template)}
          </span>
          <Button
            variant="ghost"
            size="icon"
            className="size-7"
            aria-label={`Remove ${target}`}
            onClick={() => onChange(values.filter((value) => value !== target))}
          >
            <X />
          </Button>
        </li>
      ))}
      <li className="px-3 py-2">
        {typed === undefined ? (
          <Button variant="ghost" size="sm" className="-ml-2" onClick={() => setTyped('')}>
            <Plus />
            Add a file
          </Button>
        ) : (
          <Input
            autoFocus
            aria-label="A file a worktree gets"
            placeholder="apps/api/.env"
            className="font-mono"
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            onBlur={add}
            onKeyDown={(e) => {
              if (e.key === 'Enter') add()
            }}
          />
        )}
      </li>
    </ul>
  )
}
