import { Children, Fragment, isValidElement, type ReactNode } from 'react'
import { ItemGroup, ItemSeparator } from '@/components/ui/item'

// one bordered list of shadcn Item rows, an optional header above them
export function ItemBox({
  label,
  header,
  children,
}: {
  label?: string | undefined
  header?: ReactNode
  children: ReactNode
}) {
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
