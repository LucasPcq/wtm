import { ListOrdered } from 'lucide-react'
import type { DomainField } from '@/api/types.gen'
import type { CommandForm } from '@/components/command-form/use-command-form'
import { HookList } from '@/features/worktrees/set-up/components/hook-list'
import { Setting } from '@/features/worktrees/set-up/components/setting'
import { foundIn } from '@/features/worktrees/set-up/utils/hooks'
import { foundLine, hooksRun, settingLabel } from '@/features/worktrees/set-up/utils/words'

type Hooks = Extract<DomainField, { kind: 'hooks' }>

export function HooksStep({
  form,
  onCreate,
  onClean,
  problems,
}: {
  form: CommandForm
  onCreate: Hooks
  onClean: Hooks
  problems: Record<string, string>
}) {
  return (
    <div className="flex flex-col gap-6">
      {[onCreate, onClean].map((field) => {
        const label = settingLabel[field.path === 'on_create' ? 'on_create' : 'on_clean']
        return (
          <Setting
            key={field.path}
            form={form}
            field={field}
            label={label}
            help={foundLine(foundIn(field.found))}
            problems={problems}
          >
            {({ id, values, onChange }) => (
              <HookList id={id} label={label} values={values} dirs={field.dirs} onChange={onChange} />
            )}
          </Setting>
        )
      })}

      <div className="flex flex-col gap-2">
        <span className="text-sm font-medium">How it runs</span>
        <p className="flex items-start gap-2 text-sm text-muted-foreground">
          <ListOrdered className="mt-0.5 size-4 shrink-0" />
          {hooksRun}
        </p>
      </div>
    </div>
  )
}
