import { CircleDot, CircleMinus, CircleX, LoaderCircle, type LucideIcon } from 'lucide-react'
import type { RunState } from '@/api/types.gen'
import type { Tone } from '@/features/worktrees/components/glyph'

export const jobLook: Record<RunState, { icon: LucideIcon; tone: Tone }> = {
  running: { icon: CircleDot, tone: 'ok' },
  starting: { icon: LoaderCircle, tone: 'busy' },
  crashed: { icon: CircleX, tone: 'failed' },
  exited: { icon: CircleMinus, tone: 'muted' },
  stopped: { icon: CircleMinus, tone: 'muted' },
  not_started: { icon: CircleMinus, tone: 'muted' },
}
