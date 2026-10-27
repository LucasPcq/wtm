import { Children, Fragment, isValidElement, type ReactNode } from 'react'
import { ItemGroup, ItemSeparator } from '@/components/ui/item'

// the picker's header above the rows in one box; the rows the plan leaves as they are in a section of their own, a
// sibling of the list in the body's column
export function ItemList({
  label,
  header,
  left,
  children,
}: {
  label?: string
  header?: ReactNode
  left?: ReactNode
  children: ReactNode
}) {
  const leftRows = Children.toArray(left).filter(isValidElement)

  return (
    <>
      <Box label={label} header={header}>
        {children}
      </Box>
      {leftRows.length > 0 && (
        <section aria-label="Left as they are" className="flex flex-col gap-2">
          <h4 className="text-xs font-medium text-muted-foreground">Left as they are</h4>
          <Box>{leftRows}</Box>
        </section>
      )}
    </>
  )
}

function Box({ label, header, children }: { label?: string | undefined; header?: ReactNode; children: ReactNode }) {
  const rows = Children.toArray(children).filter(isValidElement)

  return (
    // shadcn's group widens its gap when anything inside is size sm: rows touch here
    <ItemGroup aria-label={label} className="gap-0 rounded-lg border has-data-[size=sm]:gap-0 has-data-[size=xs]:gap-0">
      {header && (
        <>
          {header}
          <ItemSeparator className="my-0" />
        </>
      )}
      {rows.map((row, i) => (
        <Fragment key={row.key}>
          {i > 0 && <ItemSeparator className="my-0" />}
          {row}
        </Fragment>
      ))}
    </ItemGroup>
  )
}
