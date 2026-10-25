import type { ReactNode } from 'react'

export function PageHeader({
  title,
  description,
  action,
}: {
  title: string
  description?: ReactNode
  action?: ReactNode
}) {
  return (
    <header className="flex items-end justify-between gap-4 border-b px-8 py-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-2xl font-semibold">{title}</h1>
        <p className="min-h-5 text-sm text-muted-foreground">{description}</p>
      </div>
      {action}
    </header>
  )
}
