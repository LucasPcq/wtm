import type { ReactNode } from 'react'

export function DialogBand({ children }: { children: ReactNode }) {
  return <div className="-mx-6 border-b px-6 pb-6">{children}</div>
}
