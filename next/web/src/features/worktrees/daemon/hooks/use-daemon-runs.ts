import { create } from 'zustand'
import type { Planned } from '@/api/queries/commands'

// what a run of wtm's daemon was planned with, seen from every repository's page as the run itself is: the jobs a
// restart brings back, the plan a stop of every job follows
export const useDaemonRuns = create<{
  comingBack: number
  stopping: Planned<'daemon-stop-jobs'> | undefined
}>(() => ({ comingBack: 0, stopping: undefined }))
