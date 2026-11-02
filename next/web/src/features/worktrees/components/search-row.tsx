import { Search } from 'lucide-react'

// a row of the list box it filters, not a field above it: its icon sits in the rows' checkbox column
export function SearchRow({
  label,
  placeholder,
  query,
  onQuery,
}: {
  label: string
  placeholder: string
  query: string
  onQuery: (query: string) => void
}) {
  return (
    <label className="flex h-10 items-center gap-3 px-4 text-muted-foreground has-focus-visible:bg-muted/50">
      <Search className="size-4 shrink-0" aria-hidden="true" />
      <input
        aria-label={label}
        placeholder={placeholder}
        value={query}
        onChange={(event) => onQuery(event.target.value)}
        className="h-full min-w-0 flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground"
      />
    </label>
  )
}
