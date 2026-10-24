import type { ReactNode } from 'react'

// PageHeader is the head of every screen: its title, one line saying what it holds, and its one primary action.
export function PageHeader(props: { title: string; description?: ReactNode; action?: ReactNode }) {
  return (
    <header className="flex items-end justify-between gap-4 border-b px-8 py-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-2xl font-semibold">{props.title}</h1>
        <p className="min-h-5 text-sm text-muted-foreground">{props.description}</p>
      </div>
      {props.action}
    </header>
  )
}
