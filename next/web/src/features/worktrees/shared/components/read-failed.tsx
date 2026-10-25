import { CircleAlert } from 'lucide-react'
import { Centered } from '@/components/centered'
import { StatePage } from '@/components/state-page'

export function ReadFailed({ message }: { message: string }) {
  return (
    <Centered>
      <StatePage icon={<CircleAlert />} title="wtm couldn't read the worktrees">
        {message}
      </StatePage>
    </Centered>
  )
}
