import type { ReactNode } from 'react'

// the header pads 16 below its description, the band 4 above its content and 24 below it; an empty band is only its
// rule, 16 below the description, or the dialog's gap would leave a hole of 48
export function DialogBand({ children }: { children?: ReactNode }) {
  return <div className="-mx-6 -mt-2 border-b px-6 pt-1 pb-6 empty:p-0">{children}</div>
}
