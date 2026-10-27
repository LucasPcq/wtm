import { Children, Fragment, isValidElement, type ReactNode } from 'react'
import { ItemGroup, ItemSeparator } from '@/components/ui/item'

// one box: the picker's header above the rows, then the rows the plan leaves as they are, under their label
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
  const rows = Children.toArray(children).filter(isValidElement)
  const leftRows = Children.toArray(left).filter(isValidElement)

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
      {leftRows.length > 0 && (
        <section aria-label="Left as they are" className="contents">
          {rows.length > 0 && <ItemSeparator className="my-0" />}
          <h4 className="px-4 pt-3 pb-1 text-xs font-medium text-muted-foreground">Left as they are</h4>
          {leftRows}
        </section>
      )}
    </ItemGroup>
  )
}
