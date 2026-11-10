import { CircleMinus, Copy, File, Folder, KeyRound, Link, TriangleAlert } from 'lucide-react'
import type { EnvshowApp, EnvshowFile, EnvshowKey } from '@/api/types.gen'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Glyph } from '@/features/worktrees/components/glyph'
import { followsWords, originWords, statusWords } from '@/features/worktrees/env-files/utils/words'
import { plural } from '@/lib/plural'

const withheld = '••••••••'

export function KeysTable({ apps, worktree }: { apps: EnvshowApp[]; worktree: string }) {
  return (
    <Table aria-label="Keys by file">
      <TableHeader>
        <TableRow>
          <TableHead className="pl-14">Key</TableHead>
          <TableHead>Follows</TableHead>
          <TableHead>In main</TableHead>
          <TableHead className="text-foreground">In {worktree}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {apps.map((app) => (
          <AppRows key={app.dir} app={app} />
        ))}
      </TableBody>
    </Table>
  )
}

function AppRows({ app }: { app: EnvshowApp }) {
  const jobs = app.jobs ?? []

  const counts = () => {
    const following = `${app.following} of ${plural(app.keys, 'key')} follow a job`
    return app.to_check > 0 ? `${following} · ${app.to_check} to check` : following
  }

  return (
    <>
      <TableRow className="bg-muted/40 hover:bg-muted/40">
        <TableCell colSpan={4}>
          <div className="flex items-center gap-3">
            <Folder className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <span className="font-mono">{app.dir === '.' ? '/' : app.dir}</span>
            {jobs.length > 0 && (
              <span className="truncate text-muted-foreground">
                {jobs.join(', ')} {jobs.length === 1 ? 'runs' : 'run'} here
              </span>
            )}
            <span className="ml-auto shrink-0 text-xs text-muted-foreground">{counts()}</span>
          </div>
        </TableCell>
      </TableRow>
      {(app.files ?? []).map((file) => (
        <FileRows key={file.target} file={file} />
      ))}
    </>
  )
}

function FileRows({ file }: { file: EnvshowFile }) {
  return (
    <>
      <TableRow aria-label={file.target}>
        <TableCell colSpan={4} className="pl-8">
          <div className="flex items-center gap-2">
            <File className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <span className="font-mono">{file.name}</span>
            <span className="text-muted-foreground">
              {file.missing && file.origin !== 'none' ? 'missing: Reconcile rebuilds it, ' : ''}
              {originWords[file.origin](file.parent)}
            </span>
          </div>
        </TableCell>
      </TableRow>
      {(file.keys ?? []).map((key) => (
        <KeyRow key={key.key} envKey={key} />
      ))}
    </>
  )
}

function KeyRow({ envKey }: { envKey: EnvshowKey }) {
  const status = statusWords[envKey.status]

  const shown = (value: string | null) => {
    if (value === null) return '—'
    return envKey.withheld ? withheld : value
  }

  const follows = () => {
    if (envKey.follows === 'nothing')
      return (
        <Glyph icon={TriangleAlert} tone="attention">
          {followsWords.nothing(envKey)}
        </Glyph>
      )
    if (envKey.follows === 'copied')
      return (
        <Glyph icon={envKey.status === 'orphan' ? CircleMinus : Copy} tone="muted">
          {status ?? followsWords.copied(envKey)}
        </Glyph>
      )
    return (
      <Glyph icon={Link} tone="muted">
        {followsWords[envKey.follows](envKey)}
      </Glyph>
    )
  }

  return (
    <TableRow aria-label={envKey.key}>
      <TableCell className="pl-14">
        <span className="flex items-center gap-2 font-mono">
          <KeyRound className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
          {envKey.key}
        </span>
      </TableCell>
      <TableCell>{follows()}</TableCell>
      <TableCell className="max-w-64 truncate font-mono text-muted-foreground" title={shown(envKey.main)}>
        {shown(envKey.main)}
      </TableCell>
      <TableCell className="max-w-72 font-mono">
        <span className="block truncate" title={shown(envKey.here)}>
          {shown(envKey.here)}
          {envKey.out_of_step && envKey.writes !== undefined && (
            <span className="text-amber-600 dark:text-amber-400"> → {envKey.writes}</span>
          )}
        </span>
      </TableCell>
    </TableRow>
  )
}
