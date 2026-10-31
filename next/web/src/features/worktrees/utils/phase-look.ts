import { CircleCheck, CircleDashed, CircleX, LoaderCircle, type LucideIcon } from 'lucide-react'
import type { Tone } from '@/features/worktrees/components/glyph'
import type { PhaseRun } from '@/features/worktrees/utils/progress'

export const phaseLook: Record<PhaseRun['state'], { icon: LucideIcon; tone: Tone }> = {
  todo: { icon: CircleDashed, tone: 'muted' },
  now: { icon: LoaderCircle, tone: 'busy' },
  done: { icon: CircleCheck, tone: 'ok' },
  failed: { icon: CircleX, tone: 'failed' },
}
