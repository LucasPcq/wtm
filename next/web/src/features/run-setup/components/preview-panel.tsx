import { GitBranch } from 'lucide-react'
import type { ReactNode } from 'react'
import { Badge } from '@/components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { Open } from '@/features/run-setup/hooks/use-run-setup'
import { optionsOf, type Setup } from '@/features/run-setup/utils/setup'
import { type Part, partWords } from '@/features/run-setup/utils/words'

const Row = ({ name, children }: { name: string; children: ReactNode }) => (
  <div className="grid grid-cols-[7rem_1fr] gap-x-3 text-sm">
    <span className="truncate font-mono text-muted-foreground">{name}</span>
    <span className="flex min-w-0 flex-col">{children}</span>
  </div>
)

const Mono = ({ children }: { children: ReactNode }) => <span className="truncate font-mono">{children}</span>
const Said = ({ children }: { children: ReactNode }) => (
  <span className="text-xs text-muted-foreground">{children}</span>
)

// what a worktree gets from the answers so far: a part not reached yet is one muted line
export function PreviewPanel({
  setup,
  open,
  reached,
  onWorktree,
}: {
  setup: Setup
  open: Open
  reached: (part: Part) => boolean
  onWorktree: (worktree: string) => void
}) {
  const { form, plan } = setup
  const preview = plan.preview
  const worktrees = optionsOf(form, 'preview')

  const group = (part: Part, body: ReactNode) => {
    if (!reached(part) && open !== part)
      return (
        <p key={part} className="text-sm">
          <span className="font-medium">{partWords[part].preview}</span>{' '}
          <span className="text-muted-foreground">Decided in {partWords[part].title}</span>
        </p>
      )
    return (
      <div key={part} className="flex flex-col gap-3 border-t pt-4 first:border-t-0 first:pt-0">
        <h3 className="flex items-center gap-2 text-sm font-medium">
          {partWords[part].preview}
          {open === part && <Badge variant="outline">this part</Badge>}
        </h3>
        {body}
      </div>
    )
  }

  const step = (port: number, base: number) => {
    if (port === base) return 'the same in every worktree'
    return `${base} + ${preview.ordinal} × ${preview.spacing}`
  }

  return (
    <aside
      aria-label="Preview"
      className="sticky top-6 flex w-92 shrink-0 flex-col gap-5 self-start rounded-lg border p-6"
    >
      <header className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <span className="font-medium">What it gets in</span>
          <Select value={preview.worktree} onValueChange={onWorktree}>
            <SelectTrigger className="h-8 w-auto gap-2" aria-label="Worktree previewed">
              <GitBranch className="size-4" aria-hidden="true" />
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {worktrees.map((worktree) => (
                <SelectItem key={worktree} value={worktree}>
                  {worktree}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <p className="text-sm text-muted-foreground">
          Worked out from your answers. Nothing is written yet.
          {!preview.numbered && ` It gets worktree number ${preview.ordinal} when it first runs.`}
        </p>
      </header>

      {group(
        'runs',
        <>
          <Row name="services">
            <Mono>{(preview.services ?? []).join(', ') || '—'}</Mono>
            <Said>a copy in each worktree</Said>
          </Row>
          <Row name="tasks">
            <Mono>{(preview.tasks ?? []).join(', ') || '—'}</Mono>
            <Said>run once, before the services</Said>
          </Row>
          {(preview.joins ?? []).length > 0 && (
            <Row name="joins">
              <Mono>{(preview.joins ?? []).join(', ')}</Mono>
              <Said>shared, in main</Said>
            </Row>
          )}
        </>,
      )}
      {group(
        'shared',
        (preview.data ?? []).length > 0 ? (
          (preview.data ?? []).map((data) => (
            <Row key={data.service} name={data.service}>
              <Mono>{data.name}</Mono>
              <Said>{data.template}</Said>
            </Row>
          ))
        ) : (
          <Said>No data of its own.</Said>
        ),
      )}
      {group(
        'ports',
        (preview.ports ?? []).map((port) => (
          <Row key={`${port.job} ${port.variable}`} name={port.variable}>
            <Mono>
              {port.port} <span className="text-muted-foreground">{port.job}</span>
            </Mono>
            <Said>{port.shared ? 'shared: the same in every worktree' : step(port.port, port.base)}</Said>
          </Row>
        )),
      )}
      {group(
        'addresses',
        (preview.addresses ?? []).length > 0 ? (
          (preview.addresses ?? []).map((address) => (
            <Row key={address.job} name={address.job}>
              <Mono>{address.url}</Mono>
            </Row>
          ))
        ) : (
          <Said>Every job by its port.</Said>
        ),
      )}
      {group(
        'env',
        (preview.env ?? []).map((file) => (
          <Row key={file.file} name={file.file}>
            <Mono>{(file.keys ?? []).join(', ')}</Mono>
          </Row>
        )),
      )}
      {group(
        'profiles',
        <Row name={preview.up.name || 'every job'}>
          <Mono>{(preview.up.jobs ?? []).join(', ')}</Mono>
          <Said>{preview.up.default ? 'the default: a bare wtm run up' : 'wtm run up --profile'}</Said>
        </Row>,
      )}
    </aside>
  )
}
