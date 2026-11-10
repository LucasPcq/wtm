import { Map as MapIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { type Part, partFiles, partWords, writtenWhere } from '@/features/run-setup/utils/words'

const File = ({ name }: { name: string }) => (
  <code className="rounded border px-1.5 py-0.5 font-mono text-xs text-muted-foreground">{name}</code>
)

// the six parts as one numbered line each, with what it decides and the files it lands in
export function Overview({ shown, onStart }: { shown: Part[]; onStart: () => void }) {
  return (
    <section aria-label="Overview" className="flex flex-col gap-6 rounded-lg border p-6">
      <header className="flex gap-3">
        <MapIcon className="mt-1 size-5 shrink-0" aria-hidden="true" />
        <div className="flex flex-col gap-1">
          <h2 className="text-lg font-semibold">What you'll set up</h2>
          <p className="text-sm text-muted-foreground">Six parts, in order. Nothing is written until the end.</p>
        </div>
      </header>

      <ol className="flex flex-col gap-5">
        {shown.map((part, at) => (
          <li key={part} className="flex gap-3">
            <span className="flex size-6 shrink-0 items-center justify-center rounded-full border text-xs">
              {at + 1}
            </span>
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="font-medium">{partWords[part].title}</span>
                <span className="flex gap-1.5">
                  {partFiles[part].map((file) => (
                    <File key={file} name={file} />
                  ))}
                </span>
              </div>
              <span className="text-sm text-muted-foreground">{partWords[part].decides}</span>
            </div>
          </li>
        ))}
      </ol>

      <div className="flex flex-col gap-2 border-t pt-6 text-sm">
        <p>Then Write: the files at once, nothing before.</p>
        <p className="flex flex-wrap items-center gap-1.5 text-muted-foreground">
          <File name="run.toml" /> <File name="config.toml" /> {writtenWhere['run.toml']}
        </p>
        <p className="flex flex-wrap items-center gap-1.5 text-muted-foreground">
          <File name="docker-compose.yml" /> tracked in the repository
        </p>
      </div>

      <div className="flex justify-end">
        <Button onClick={onStart}>Start with {partWords[shown[0] ?? 'runs'].title}</Button>
      </div>
    </section>
  )
}
