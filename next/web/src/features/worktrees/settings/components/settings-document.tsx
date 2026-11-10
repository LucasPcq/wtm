import { BookmarkCheck } from 'lucide-react'
import type { ConfigshowDetail, ConfigshowRemembered, DomainField, DomainFollowUp } from '@/api/types.gen'
import { HookList } from '@/features/worktrees/settings/components/hook-list'
import { RememberedAnswers } from '@/features/worktrees/settings/components/remembered-answers'
import { SectionHead } from '@/features/worktrees/settings/components/section-head'
import { StrategyCards } from '@/features/worktrees/settings/components/strategy-cards'
import { ToCheck } from '@/features/worktrees/settings/components/to-check'
import { ValueRow } from '@/features/worktrees/settings/components/value-row'
import { shown } from '@/features/worktrees/settings/utils/shown'
import {
  isSectionId,
  isSetting,
  type SettingPath,
  type Strategy,
  sectionWords,
  settingWords,
} from '@/features/worktrees/settings/utils/words'

export function SettingsDocument({
  detail,
  edit,
  busy,
  onEdit,
  onChoose,
  onForget,
}: {
  detail: ConfigshowDetail
  edit: DomainFollowUp
  busy: boolean
  onEdit: (path: SettingPath) => void
  onChoose: (strategy: Strategy) => void
  onForget: (answer: ConfigshowRemembered) => void
}) {
  const fields = detail.settings.fields.filter(isSetting)
  const fieldOf = (path: SettingPath) => fields.find((field) => field.path === path)
  const defaultOf = (path: SettingPath) => detail.defaults?.find((d) => d.path === path)
  const problemsOf = (paths: SettingPath[]) =>
    (detail.settings.problems ?? []).flatMap((problem) => {
      const setting = paths.find((path) => problem.path === path || problem.path.startsWith(`${path}[`))
      return setting ? [{ ...problem, setting }] : []
    })
  const shownSections = (detail.settings.sections ?? []).flatMap(({ id }) => {
    if (!isSectionId(id)) return []
    const paths = fields.filter((field) => field.section === id).map((field) => field.path)
    return [{ id, paths, ...sectionWords[id] }]
  })

  const control = (path: SettingPath, field: DomainField) => {
    const words = settingWords[path]
    if (field.kind === 'select' && field.closed)
      return (
        <div key={path} className="flex flex-col gap-4">
          <StrategyCards
            field={field}
            defaultValue={defaultOf(path)?.value}
            disabled={busy || detail.newer}
            onChoose={onChoose}
          />
        </div>
      )
    if (field.kind === 'hooks' && (path === 'on_create' || path === 'on_clean'))
      return <HookList key={path} field={field} list={path} label={words.label} onChange={() => onEdit(path)} />
    const help = field.kind === 'multi_select' && field.picked.length > 0 ? field.picked.join(', ') : words.help
    return (
      <ValueRow
        key={path}
        label={words.label}
        help={help}
        value={shown(field)}
        isDefault={defaultOf(path)?.in_use ?? false}
        onChange={() => onEdit(path)}
      />
    )
  }

  return (
    <div className="flex flex-col px-8">
      {shownSections.map((section) => (
        <section key={section.id} className="grid grid-cols-[280px_1fr] gap-8 border-b py-6 last:border-b-0">
          <SectionHead icon={section.icon} title={section.title} line={section.line} />
          <div className="flex flex-col gap-4">
            <ToCheck problems={problemsOf(section.paths)} />
            {section.paths.flatMap((path) => {
              const field = fieldOf(path)
              return field ? [control(path, field)] : []
            })}
          </div>
        </section>
      ))}
      <section className="grid grid-cols-[280px_1fr] gap-8 py-6">
        <SectionHead
          icon={BookmarkCheck}
          title="Remembered answers"
          line="Create's questions answered with “Always use this answer”."
        />
        <RememberedAnswers
          remembered={detail.remembered ?? []}
          edit={edit}
          disabled={busy || detail.newer}
          onForget={onForget}
        />
      </section>
    </div>
  )
}
