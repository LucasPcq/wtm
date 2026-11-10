import { create } from 'zustand'
import type { Planned } from '@/api/queries/commands'

// what a run of wtm's daemon was planned with, seen from every repository's page as the run itself is: the jobs a
// restart brings back, the plan a stop of every job follows, whether that stop was hidden while it ran, and the dialog
// open, which a toast opens again from wherever it shows
export const useDaemonRuns = create<{
  comingBack: number
  stopping: Planned<'daemon-stop-jobs'> | undefined
  hidden: boolean
  asking: 'restart' | 'stop' | undefined
}>(() => ({ comingBack: 0, stopping: undefined, hidden: false, asking: undefined }))
