import { Children, Fragment, isValidElement, type ReactNode, useId } from 'react'
import { ItemGroup, ItemSeparator } from '@/components/ui/item'

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
  const leftHeading = useId()

  return (
    <>
      <Box label={label} header={header}>
        {children}
      </Box>
      {leftRows.length > 0 && (
        <section aria-labelledby={leftHeading} className="flex flex-col gap-2">
          <h3 id={leftHeading} className="text-xs font-medium text-muted-foreground">
            Left as they are
          </h3>
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
