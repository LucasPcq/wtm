import type { ReactNode } from 'react'

// empty, the band is only its rule: its padding on the dialog's gap would leave 48 under the description
export function DialogBand({ children }: { children?: ReactNode }) {
  return <div className="-mx-6 -mt-2 border-b px-6 pt-1 pb-6 empty:p-0">{children}</div>
}
