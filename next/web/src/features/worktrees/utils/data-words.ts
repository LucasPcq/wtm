import type {
  TeardownDataReason,
  TeardownDataStatus,
  TeardownDropped,
  TeardownDropStatus,
  TeardownNamespace,
} from '@/api/types.gen'

const keptWords: Record<TeardownDataReason, string> = {
  keep_data: 'stays: its data is kept',
  shared_name: 'stays: another worktree reaches it under the same name',
  service_down: 'stays until its service is up',
}

const keptWhy = ({ reason }: { reason?: TeardownDataReason }) => keptWords[reason ?? 'keep_data']

export const dataWords: Record<TeardownDataStatus, (namespace: TeardownNamespace) => string> = {
  drop: () => 'is dropped',
  start: ({ job }) => `is dropped: ${job} is started for it, then stopped`,
  defer: ({ job }) => `stays until ${job} is up: it is down`,
  keep: keptWhy,
}

// a drop that failed is said by its warning, under "To check"
export const droppedWords: Record<TeardownDropStatus, ((dropped: TeardownDropped) => string) | null> = {
  dropped: () => 'dropped',
  deferred: ({ job }) => `stays until ${job} is up, then the next run up or prune drops it`,
  kept: keptWhy,
  failed: null,
}
