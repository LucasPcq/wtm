import { Children, Fragment, isValidElement, type ReactNode } from 'react'
import { ItemGroup, ItemSeparator } from '@/components/ui/item'

export function ItemList({ label, children }: { label?: string; children: ReactNode }) {
  const rows = Children.toArray(children).filter(isValidElement)

  return (
    <ItemGroup aria-label={label} className="gap-0 rounded-lg border">
      {rows.map((row, i) => (
        <Fragment key={row.key}>
          {i > 0 && <ItemSeparator className="my-0" />}
          {row}
        </Fragment>
      ))}
    </ItemGroup>
  )
}
