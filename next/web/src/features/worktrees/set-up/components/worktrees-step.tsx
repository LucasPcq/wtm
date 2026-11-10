import type { DomainField } from '@/api/types.gen'
import type { CommandForm } from '@/components/command-form/use-command-form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Setting } from '@/features/worktrees/set-up/components/setting'
import { branchHelp, folderHelp, settingLabel } from '@/features/worktrees/set-up/utils/words'

export function WorktreesStep({
  form,
  name,
  folder,
  branch,
  problems,
}: {
  form: CommandForm
  name: string
  folder: Extract<DomainField, { kind: 'text' }>
  branch: Extract<DomainField, { kind: 'select' }>
  problems: Record<string, string>
}) {
  return (
    <div className="flex flex-col gap-6">
      <Setting
        form={form}
        field={folder}
        label={settingLabel.base_path}
        help={folder.origin && folderHelp(name)[folder.origin]}
        problems={problems}
      >
        {({ id, values, onChange, invalid }) => (
          <Input
            id={id}
            className="font-mono"
            aria-invalid={invalid}
            value={values[0] ?? ''}
            onChange={(e) => onChange([e.target.value])}
          />
        )}
      </Setting>

      <Setting
        form={form}
        field={branch}
        label={settingLabel.base_branch}
        help={branch.origin && branchHelp[branch.origin]}
        problems={problems}
      >
        {({ id, values, onChange, invalid }) => (
          <Select value={values[0] ?? ''} onValueChange={(value) => onChange([value])}>
            <SelectTrigger id={id} aria-invalid={invalid} className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {branch.options.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.value}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </Setting>
    </div>
  )
}
