import type { ReactNode } from 'react'

export function Centered(props: { children: ReactNode }) {
  return <div className="flex flex-1 items-center justify-center p-8">{props.children}</div>
}
