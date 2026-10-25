import { CircleAlert } from 'lucide-react'
import { Centered } from '@/components/centered'
import { StatePage } from '@/components/state-page'
import { Skeleton } from '@/components/ui/skeleton'

export function Loading() {
  return (
    <div className="flex flex-col">
      {[220, 160, 240, 180, 200].map((width) => (
        <div key={width} className="flex h-14 items-center gap-6 border-b px-5">
          <div className="flex flex-1 flex-col gap-1.5">
            <Skeleton className="h-3" style={{ width }} />
            <Skeleton className="h-2.5 w-20" />
          </div>
          <Skeleton className="h-3 w-24" />
          <Skeleton className="h-3 w-32" />
          <Skeleton className="h-3 w-16" />
        </div>
      ))}
    </div>
  )
}

export function ReadFailed({ message }: { message: string }) {
  return (
    <Centered>
      <StatePage icon={<CircleAlert />} title="wtm couldn't read the worktrees">
        {message}
      </StatePage>
    </Centered>
  )
}
