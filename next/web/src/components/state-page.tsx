import type { ReactNode } from 'react'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Logo } from './logo'

// StatePage is a state the page stands in rather than a screen: what happened and the one way out of it.
export function StatePage(props: { icon?: ReactNode; title: string; children: ReactNode; command?: string }) {
  return (
    <Empty className="gap-6">
      <EmptyHeader className="max-w-md gap-6">
        {props.icon ? <EmptyMedia variant="icon" className="mb-0 size-12 rounded-[10px] [&_svg:not([class*='size-'])]:size-6">
            {props.icon}
          </EmptyMedia> : <Logo />}
        <div className="flex flex-col items-center gap-2">
          <EmptyTitle className="text-lg font-semibold">{props.title}</EmptyTitle>
          <EmptyDescription>{props.children}</EmptyDescription>
        </div>
      </EmptyHeader>
      {props.command && (
        <EmptyContent>
          <code className="rounded-md bg-muted px-3 py-2 font-mono text-sm">{props.command}</code>
        </EmptyContent>
      )}
    </Empty>
  )
}
