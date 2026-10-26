import { Children, Fragment, isValidElement, type ReactNode } from 'react'
import { ItemGroup, ItemSeparator } from '@/components/ui/item'

export function ItemList({ label, children }: { label?: string; children: ReactNode }) {
  const rows = Children.toArray(children).filter(isValidElement)

  return (
    // shadcn's group widens its gap when anything inside is size sm, the step's toggle among them: rows touch here
    <ItemGroup aria-label={label} className="gap-0 rounded-lg border has-data-[size=sm]:gap-0 has-data-[size=xs]:gap-0">
      {rows.map((row, i) => (
        <Fragment key={row.key}>
          {i > 0 && <ItemSeparator className="my-0" />}
          {row}
        </Fragment>
      ))}
    </ItemGroup>
  )
}
