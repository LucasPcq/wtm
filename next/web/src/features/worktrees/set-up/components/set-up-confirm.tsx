import { FolderCheck } from 'lucide-react'
import type { ReactNode } from 'react'
import type { ProjectHook, SettingsDocument } from '@/api/types.gen'
import { hookWhere, isStrategy, settingLabel, strategyCard } from '@/features/worktrees/set-up/utils/words'

// the settings as the dry run settles them: what is written, word for word
export function SetUpConfirm({ name, settings }: { name: string; settings: SettingsDocument }) {
  const strategy = isStrategy(settings.env_strategy) ? strategyCard[settings.env_strategy].title : settings.env_strategy
  const files = settings.env_files ?? []

  return (
    <div className="flex flex-col gap-6">
      <dl className="grid grid-cols-[minmax(0,14rem)_1fr] gap-x-6 gap-y-2 text-sm">
        {[
          [settingLabel.base_path, settings.base_path],
          [settingLabel.base_branch, settings.base_branch],
          [settingLabel.env_strategy, strategy],
        ].map(([label, value]) => (
          <div key={label} className="contents">
            <dt className="text-muted-foreground">{label}</dt>
            <dd className="min-w-0 truncate">{value}</dd>
          </div>
        ))}
      </dl>

      {files.length > 0 && (
        <Listed label={settingLabel.env_files}>
          {files.map((file) => (
            <li key={file.target} className="truncate px-4 py-2.5 font-mono text-sm">
              {file.target}
            </li>
          ))}
        </Listed>
      )}
      <Hooks label={`${settingLabel.on_create}, in order`} hooks={settings.on_create ?? []} />
      <Hooks label={settingLabel.on_clean} hooks={settings.on_clean ?? []} />

      <div className="flex flex-col gap-2">
        <span className="text-sm font-medium">How it runs</span>
        <p className="flex items-start gap-2 text-sm text-muted-foreground">
          <FolderCheck className="mt-0.5 size-4 shrink-0" />
          Then the web knows {name} and opens on its worktrees.
        </p>
      </div>
    </div>
  )
}

function Listed({ label, children }: { label: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-2">
      <h4 className="text-sm font-medium">{label}</h4>
      <ul aria-label={label} className="divide-y rounded-lg border">
        {children}
      </ul>
    </section>
  )
}

function Hooks({ label, hooks }: { label: string; hooks: ProjectHook[] }) {
  if (hooks.length === 0) return null

  return (
    <Listed label={label}>
      {hooks.map((hook, i) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: a hook is its place in the list, the same command may run twice
        <li key={i} className="flex items-center gap-3 px-4 py-2.5 text-sm">
          <span className="w-4 shrink-0 text-muted-foreground">{i + 1}</span>
          <span className="min-w-0 flex-1 truncate font-mono">{hook.cmd}</span>
          {hook.continue_on_error && (
            <span className="shrink-0 text-xs text-muted-foreground">goes on if it fails</span>
          )}
          <span className="shrink-0 text-xs text-muted-foreground">{hookWhere(hook.cwd)}</span>
        </li>
      ))}
    </Listed>
  )
}
