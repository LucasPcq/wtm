import type { ReactNode } from 'react'

export function Centered({ children }: { children: ReactNode }) {
  return <div className="flex flex-1 items-center justify-center p-8">{children}</div>
}
