import { Children, isValidElement, type ReactNode } from 'react'
import { ItemBox } from '@/features/worktrees/components/item-box'
import { LeftSection } from '@/features/worktrees/components/left-section'

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
  const listed = header !== undefined || Children.toArray(children).some(isValidElement)

  return (
    <>
      {listed && (
        <ItemBox label={label} header={header}>
          {children}
        </ItemBox>
      )}
      <LeftSection>{left}</LeftSection>
    </>
  )
}
