import { useQuery } from '@tanstack/react-query'
import { ListOrdered, CircleX as Stops } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { HttpError } from '@/api/fetch'
import { questionsQuery } from '@/api/queries/commands'
import type { DomainAnswered, DomainError, DomainForm } from '@/api/types.gen'
import { defaultOf, keyOf, spellHook } from '@/components/command-form/answers'
import { FieldInput } from '@/components/command-form/field-input'
import { useCommandForm } from '@/components/command-form/use-command-form'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert } from '@/features/worktrees/components/alert'
import { CopyCommand } from '@/features/worktrees/components/copy-command'
import { HooksEditor } from '@/features/worktrees/settings/components/hooks-editor'
import { useSaveSetting } from '@/features/worktrees/settings/hooks/use-save-setting'
import {
  fieldProblem,
  hookWords,
  type SettingPath,
  settingWords,
  strategyWords,
} from '@/features/worktrees/settings/utils/words'
import { cn } from '@/lib/utils'

export type Editing = { path: SettingPath; digest: string; newer: boolean; onDisk: string }

export function ChangeDialog({
  repo,
  editing,
  onClose,
}: {
  repo: string
  editing: Editing | undefined
  onClose: () => void
}) {
  return (
    <Dialog open={!!editing} onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        className={cn(
          'flex max-h-[90vh] flex-col gap-6 p-6 sm:max-w-2xl',
          // the house frame: 24 around, the header and the footer ruled off from the body
          '[&_[data-slot=dialog-header]]:-mx-6 [&_[data-slot=dialog-header]]:gap-1 [&_[data-slot=dialog-header]]:border-b [&_[data-slot=dialog-header]]:px-6 [&_[data-slot=dialog-header]]:pb-6',
          '[&_[data-slot=dialog-footer]]:-mx-6 [&_[data-slot=dialog-footer]]:-mb-6 [&_[data-slot=dialog-footer]]:px-6 [&_[data-slot=dialog-footer]]:py-4 [&>button.absolute]:top-4 [&>button.absolute]:right-4',
        )}
        // a toast's button (Undo) is clicked over the dialog: it keeps the dialog open
        onInteractOutside={(event) => {
          if (event.target instanceof Element && event.target.closest('[data-sonner-toaster]')) event.preventDefault()
        }}
      >
        {editing && <Change key={editing.path} repo={repo} editing={editing} onClose={onClose} />}
      </DialogContent>
    </Dialog>
  )
}

function Change({ repo, editing, onClose }: { repo: string; editing: Editing; onClose: () => void }) {
  const form = useQuery({ ...questionsQuery(repo, 'config-set'), staleTime: Infinity, refetchOnWindowFocus: false })
  const label = settingWords[editing.path].label

  return (
    <>
      <DialogHeader>
        <DialogTitle>Change a setting</DialogTitle>
        <DialogDescription>{label}</DialogDescription>
      </DialogHeader>
      {form.data && <Editor repo={repo} editing={editing} initial={form.data} onClose={onClose} />}
      {form.isPending && <Skeleton className="h-10 w-full" />}
      {form.isError && <Alert tone="red" title="Couldn't read the setting" description={form.error.message} />}
    </>
  )
}

function Editor({
  repo,
  editing,
  initial,
  onClose,
}: {
  repo: string
  editing: Editing
  initial: DomainForm
  onClose: () => void
}) {
  const { form, asked, answers, check } = useCommandForm({ repo, command: 'config-set', form: initial })
  const { save, saving } = useSaveSetting(repo)
  const [refusal, setRefusal] = useState<DomainError>()
  // a command row just added and not yet typed is no command: Save waits for it
  const [incomplete, setIncomplete] = useState(false)
  const field = asked.fields.find(({ path }) => path === editing.path)
  const key = keyOf(editing.path)
  const said = settingWords[editing.path]
  const changedOnDisk = refusal?.code === 'config-set.changed_on_disk'
  const own = (path: string) => path === editing.path || path.startsWith(`${editing.path}[`)
  const problem = [...asked.problems, ...(refusal?.fields ?? [])]
    .filter(({ path }) => own(path))
    .map(fieldProblem)
    .join('\n')

  if (!field) return null

  const submit = async (mine: boolean) => {
    const sent: DomainAnswered[] = answers().filter(({ path }) => path === editing.path)
    const made = mine ? [] : [{ path: 'replaces', values: [editing.digest] }]
    try {
      await save([...sent, ...made])
      toast.success('Saved', { description: said.label })
      onClose()
    } catch (failed) {
      if (failed instanceof HttpError && failed.refusal) return setRefusal(failed.refusal)
      setRefusal({ kind: 'internal', code: 'web.failed', message: String(failed) })
    }
  }

  const options = Object.fromEntries(
    Object.entries(strategyWords).map(([value, { title, line }]) => [value, { title, description: line }]),
  )
  const primary = () => {
    if (saving) return 'Saving'
    return changedOnDisk ? 'Save mine' : 'Save'
  }

  const input = (value: string[], change: (values: string[]) => void) => {
    if (field.kind === 'hooks')
      return (
        <HooksEditor
          entries={field.entries}
          dirs={field.dirs}
          disabled={editing.newer || saving}
          onChange={(hooks) => {
            setIncomplete(hooks.some(({ cmd }) => !cmd.trim()))
            change(hooks.map(spellHook))
          }}
        />
      )
    return (
      <fieldset disabled={editing.newer || saving}>
        <FieldInput
          field={field}
          id={key}
          words={{ label: said.label, help: said.help, options }}
          from={undefined}
          values={value}
          onChange={change}
          invalid={!!problem}
        />
      </fieldset>
    )
  }

  return (
    <>
      <div className="flex flex-col gap-4">
        {editing.newer && (
          <Alert
            tone="red"
            title="Written by a newer wtm"
            description="This wtm reads these settings but can't write them. Upgrade it to change them here."
            below={<CopyCommand text="wtm upgrade" />}
          />
        )}
        {changedOnDisk && (
          <Alert
            tone="amber"
            title="Changed on disk while you were editing"
            description={`It reads ${editing.onDisk} there now. Save writes yours over it.`}
          />
        )}
        {refusal && !changedOnDisk && !refusal.fields && (
          <Alert tone="red" title="Couldn't save" description={refusal.message} />
        )}
        <form.Field name={key} defaultValue={defaultOf(field)}>
          {(state) => (
            <div className="flex flex-col gap-2">
              {input(state.state.value, (values) => {
                state.handleChange(values)
                void check().catch(() => undefined)
              })}
              {problem && <p className="text-sm text-destructive">{problem}</p>}
              {field.kind !== 'hooks' && <p className="text-sm text-muted-foreground">{said.help}</p>}
            </div>
          )}
        </form.Field>
        {(editing.path === 'on_create' || editing.path === 'on_clean') && (
          <div className="flex flex-col gap-2 text-sm">
            <span className="text-xs font-medium text-muted-foreground">How it runs</span>
            <span className="flex items-center gap-2">
              <ListOrdered className="size-4 text-muted-foreground" aria-hidden="true" />
              {editing.path === 'on_create'
                ? 'In order, in the new worktree, once its .env files are written.'
                : 'In order, in the worktree, before it is removed.'}
            </span>
            <span className="flex items-center gap-2">
              <Stops className="size-4 text-muted-foreground" aria-hidden="true" />
              {hookWords[editing.path].fails}
            </span>
          </div>
        )}
      </div>
      <DialogFooter className="border-t sm:justify-between">
        <Button variant="ghost" onClick={onClose}>
          Cancel
        </Button>
        <div className="flex gap-2">
          {changedOnDisk && (
            <Button variant="outline" onClick={onClose}>
              Keep theirs
            </Button>
          )}
          <Button
            disabled={editing.newer || saving || incomplete || asked.problems.some(({ path }) => own(path))}
            onClick={() => void submit(changedOnDisk)}
          >
            {primary()}
          </Button>
        </div>
      </DialogFooter>
    </>
  )
}
