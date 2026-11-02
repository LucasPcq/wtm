// a grid, not flex: each column keeps its whole text while both fit, and past that at least half the row, so a short
// line ("onto main · 1 commit") stays whole and a long name gives way; each cut shows its full text on hover
export function NameLine({ name, line }: { name: string; line: string | undefined }) {
  return (
    <span className="grid min-w-0 flex-1 grid-cols-[minmax(0,max-content)_minmax(0,max-content)] items-center gap-3">
      <span title={name} className="truncate font-medium">
        {name}
      </span>
      <span title={line} className="truncate text-muted-foreground">
        {line}
      </span>
    </span>
  )
}
