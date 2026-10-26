import { cn } from '@/lib/utils'

export type Fact = { label: string; values: string[]; mono?: boolean; line?: string }

// one subject's facts inside a Details: a label, then each value on its line
export function Facts({ heading, facts }: { heading?: string | undefined; facts: Fact[] }) {
  return (
    <section aria-label={heading} className="flex flex-col gap-2 text-xs">
      {heading && <h4 className="font-medium">{heading}</h4>}
      <dl className="grid grid-cols-[6rem_1fr] gap-x-6 gap-y-2">
        {facts.map(({ label, values, mono, line }) => (
          <div key={label} className="contents">
            <dt className="text-muted-foreground">{label}</dt>
            <dd className="min-w-0">
              {values.map((value) => (
                <span key={value} className={cn('block', mono ? 'font-mono break-all' : 'text-pretty')}>
                  {value}
                </span>
              ))}
              {line && <span className="block text-muted-foreground">{line}</span>}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  )
}
