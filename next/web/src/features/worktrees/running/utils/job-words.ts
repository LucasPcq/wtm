import { LogOut, type LucideIcon, Play, RotateCcw, Square } from 'lucide-react'
import { holdOf, type JobAt } from '@/features/worktrees/utils/job-told'

export type JobAct = 'restart' | 'stop' | 'release' | 'start'

export const actLook: Record<JobAct, { icon: LucideIcon; label: string }> = {
  restart: { icon: RotateCcw, label: 'Restart' },
  stop: { icon: Square, label: 'Stop' },
  release: { icon: LogOut, label: 'Release' },
  start: { icon: Play, label: 'Start' },
}

// the menu says whose hold a release lets go of, and what keeps running
export function actEntry(
  act: JobAct,
  { at, keptBy, holders }: { at: JobAt; keptBy: string[]; holders: number | undefined },
): { label: string; description?: string } {
  if (act === 'restart' && holders !== undefined)
    return { label: `Restart ${at.job}`, description: `for the ${holders} worktrees holding it` }
  if (act !== 'release') return { label: actLook[act].label }
  return {
    label: `Release ${at.worktree}’s hold`,
    description: `${at.job} keeps running for ${keptBy.length === 1 ? 'the other' : `the ${keptBy.length} others`}`,
  }
}

// a start, a stop or a release of the job, as its row and its header say it while it runs
export type JobDoing = 'restarting' | 'starting' | 'stopping' | 'releasing'

export const doingWords: Record<JobDoing, string> = {
  restarting: 'restarting…',
  starting: 'starting…',
  stopping: 'stopping…',
  releasing: 'releasing…',
}

export function doingToast(doing: JobDoing, at: JobAt): { title: string; where: string } {
  if (doing === 'restarting') return { title: `Restarting ${at.job}`, where: `${at.worktree} · its log goes on below` }
  if (doing === 'starting') return { title: `Starting ${at.job}`, where: at.worktree }
  if (doing === 'releasing') return { title: `Releasing ${holdOf(at)}`, where: at.worktree }
  return { title: `Stopping ${at.job}`, where: at.worktree }
}
