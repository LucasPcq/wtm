import { FileText } from 'lucide-react'
import type { DomainWarning, RuninitEnvKey, RuninitLinkRow, RuninitPreview } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { CheckRow, ToCheck } from '@/features/run-setup/components/to-check'
import { type PartProps, pickedOf, toggled } from '@/features/run-setup/utils/setup'
import { kindWords } from '@/features/run-setup/utils/words'
import { cn } from '@/lib/utils'

export const envNotCopied = 'run-init.env_file_not_copied'

// a key's value in the worktree previewed: the port, the address or the template it follows, else as main holds it
function valueIn(preview: RuninitPreview, link: RuninitLinkRow | undefined, key: RuninitEnvKey) {
  if (!link) return key.value
  if (link.kind === 'port')
    return String(preview.ports?.find((p) => p.job === link.job && p.variable === link.port)?.port ?? key.value)
  if (link.kind === 'address') return preview.addresses?.find((a) => a.job === link.job)?.url ?? key.value
  return link.template ?? key.value
}

export function EnvPart({ setup, answer, warnings }: PartProps & { warnings: DomainWarning[] }) {
  const { form, plan } = setup
  const linked = pickedOf(form, 'links')
  const copied = pickedOf(form, 'env_copy')
  const notCopied = warnings.filter(({ code }) => code === envNotCopied)

  return (
    <div className="flex flex-col gap-6">
      <ToCheck count={notCopied.length}>
        {notCopied.map(({ params = {} }) => (
          <CheckRow
            key={params.file}
            subject={params.file ?? ''}
            said="not copied into worktrees"
            action={
              <Button variant="outline" size="sm" onClick={() => answer('env_copy', [...copied, params.file ?? ''])}>
                Copy it
              </Button>
            }
          >
            <p className="text-muted-foreground">Worktrees don't get this file. Copy it: config.toml copies it in.</p>
          </CheckRow>
        ))}
      </ToCheck>

      <table className="w-full overflow-hidden rounded-md border text-sm">
        <thead className="bg-muted/50 text-left text-muted-foreground">
          <tr>
            <th className="w-10 px-4 py-2" />
            <th className="px-2 py-2 font-normal">Key</th>
            <th className="px-2 py-2 font-normal">Job</th>
            <th className="px-2 py-2 font-normal">Kind</th>
            <th className="px-2 py-2 font-normal">In {plan.preview.worktree}</th>
          </tr>
        </thead>
        {(plan.detection.env_files ?? []).map((file) => {
          const uncopied = !file.copied && !copied.includes(file.file)
          return (
            <tbody key={file.file}>
              <tr className="border-t bg-muted/30">
                <th colSpan={4} className="px-4 py-2 text-left font-normal">
                  <span className="flex items-center gap-2 font-mono">
                    <FileText className="size-4" aria-hidden="true" />
                    {file.file}
                  </span>
                </th>
                <td className="px-2 py-2 text-right text-muted-foreground">{uncopied ? 'not copied' : 'copied'}</td>
              </tr>
              {(file.keys ?? []).map((key) => {
                const link = plan.parts.links.find((row) => row.file === file.file && row.key === key.key)
                return (
                  <tr key={key.key} className={cn('border-t', uncopied && 'text-muted-foreground')}>
                    <td className="px-4 py-2">
                      {link && (
                        <Checkbox
                          aria-label={`${file.file} ${key.key}`}
                          checked={linked.includes(link.spelled)}
                          onCheckedChange={(on) => answer('links', toggled(linked, link.spelled, on === true))}
                        />
                      )}
                    </td>
                    <td className="px-2 py-2 font-mono">{key.key}</td>
                    <td className="px-2 py-2">{key.job ?? '—'}</td>
                    <td className="px-2 py-2">{kindWords[key.kind ?? 'none'] ?? key.kind}</td>
                    <td className="max-w-48 truncate px-2 py-2 font-mono text-xs">
                      {uncopied ? 'once copied' : valueIn(plan.preview, link, key)}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          )
        })}
      </table>
    </div>
  )
}
