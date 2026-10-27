import type { ReactNode } from 'react'
import { Logo } from '@/components/logo'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'

export function StatePage({
  icon,
  title,
  children,
  command,
  action,
}: {
  icon?: ReactNode
  title: string
  children: ReactNode
  command?: string
  action?: ReactNode
}) {
  return (
    <Empty className="gap-6">
      <EmptyHeader className="max-w-md gap-6">
        {icon ? (
          <EmptyMedia variant="icon" className="mb-0 size-12 rounded-[10px] [&_svg:not([class*='size-'])]:size-6">
            {icon}
          </EmptyMedia>
        ) : (
          <Logo />
        )}
        <div className="flex flex-col items-center gap-2">
          <EmptyTitle className="text-lg font-semibold">{title}</EmptyTitle>
          <EmptyDescription>{children}</EmptyDescription>
        </div>
      </EmptyHeader>
      {(command || action) && (
        <EmptyContent>
          {command && <code className="rounded-md bg-muted px-3 py-2 font-mono text-sm">{command}</code>}
          {action}
        </EmptyContent>
      )}
    </Empty>
  )
}
