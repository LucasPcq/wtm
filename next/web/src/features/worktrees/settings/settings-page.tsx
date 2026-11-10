import { useQuery } from '@tanstack/react-query'
import { Copy, FileX, Settings } from 'lucide-react'
import { useState } from 'react'
import { HttpError } from '@/api/fetch'
import { commandLineQuery } from '@/api/queries/command-line'
import { readQuery } from '@/api/queries/commands'
import { reposQuery } from '@/api/queries/repos'
import type { ConfigshowRemembered, DomainFollowUp } from '@/api/types.gen'
import { PageHeader } from '@/components/page-header'
import { StatePage } from '@/components/state-page'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { ChangeDialog, type Editing } from '@/features/worktrees/settings/components/change-dialog'
import { SettingsDocument } from '@/features/worktrees/settings/components/settings-document'
import { useSaveSetting } from '@/features/worktrees/settings/hooks/use-save-setting'
import { shown } from '@/features/worktrees/settings/utils/shown'
import {
  isSetting,
  isSettingPath,
  refusalWords,
  rememberedWords,
  type SettingPath,
  type Strategy,
  settingWords,
} from '@/features/worktrees/settings/utils/words'
import { copy } from '@/features/worktrees/utils/copy'
import { repoName } from '@/lib/repo'

const edit: DomainFollowUp = { command: 'config-edit', code: 'config.edit', message: 'fix it in your editor' }
const init: DomainFollowUp = { command: 'init', code: 'repository.init', message: 'set wtm up in this repository' }

export function SettingsPage({ repo }: { repo: string }) {
  const root = useQuery(reposQuery).data?.find((known) => known.id === repo)?.root
  // config.toml edited by hand sends no event: coming back to the page from the editor reads it again
  const read = useQuery({ ...readQuery(repo, 'config-show'), refetchOnWindowFocus: true })
  const editLine = useQuery(commandLineQuery(edit)).data
  const { atOnce, saving } = useSaveSetting(repo)
  // the digest is the file's as the dialog opened on it: a change on disk meanwhile is then refused, not written over
  const [editing, setEditing] = useState<{ path: SettingPath; digest: string }>()
  const detail = read.data?.items[0]?.detail
  const refused = read.error instanceof HttpError ? read.error.refusal : undefined

  const header = (action?: boolean) => (
    <PageHeader
      title="Settings"
      description={`Where ${root ? repoName(root) : 'the repository'}'s worktrees go, how they are made and removed`}
      action={
        action &&
        editLine && (
          <Button variant="outline" onClick={() => copy(editLine)}>
            <Copy />
            Edit in your editor
          </Button>
        )
      }
    />
  )

  if (refused?.code === 'config-show.config_not_found')
    return (
      <>
        {header()}
        <StatePage icon={<Settings />} title={refusalWords.notFound.title} action={<CommandLine followUp={init} />}>
          {refusalWords.notFound.line}
        </StatePage>
      </>
    )

  if (refused?.code === 'config-show.config_invalid') {
    const setting = refused.params?.setting
    const where = [
      refused.params?.line && `Line ${refused.params.line}`,
      setting && isSettingPath(setting) && `“${settingWords[setting].label}”`,
    ].filter(Boolean)
    return (
      <>
        {header()}
        <StatePage icon={<FileX />} title={refusalWords.noParse.title} action={<CommandLine followUp={edit} />}>
          {where.length > 0 ? `${where.join(': ')} doesn't parse. ` : ''}
          {refusalWords.noParse.fix}
        </StatePage>
      </>
    )
  }

  if (!detail)
    return (
      <>
        {header()}
        <div role="status" className="flex flex-col gap-6 px-8 py-6" aria-label="Loading the settings">
          {[0, 1, 2].map((at) => (
            <Skeleton key={at} className="h-24 w-full" />
          ))}
        </div>
      </>
    )

  const opened = ({ path, digest }: { path: SettingPath; digest: string }): Editing => {
    const field = detail.settings.fields.filter(isSetting).find((f) => f.path === path)
    return { path, digest, current: detail.digest, newer: detail.newer, onDisk: field ? shown(field) : '' }
  }

  const choose = (strategy: Strategy) =>
    atOnce(settingWords.env_strategy.label, [
      { path: 'env_strategy', values: [strategy] },
      { path: 'replaces', values: [detail.digest] },
    ])
  const forget = (answer: ConfigshowRemembered) =>
    atOnce(`Forgot: ${rememberedWords[answer.question]?.question ?? answer.question}`, [
      {
        path: 'remembered',
        values: (detail.remembered ?? []).filter(({ entry }) => entry !== answer.entry).map(({ entry }) => entry),
      },
      { path: 'replaces', values: [detail.digest] },
    ])

  return (
    <>
      {header(true)}
      <div className="overflow-y-auto">
        <SettingsDocument
          detail={detail}
          edit={edit}
          busy={saving}
          onEdit={(path) => setEditing({ path, digest: detail.digest })}
          onChoose={choose}
          onForget={forget}
        />
      </div>
      <ChangeDialog repo={repo} editing={editing && opened(editing)} onClose={() => setEditing(undefined)} />
    </>
  )
}
