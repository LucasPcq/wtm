import {
  FastForward,
  FolderOpen,
  GitBranch,
  GitPullRequest,
  type LucideIcon,
  Play,
  RefreshCw,
  ScrollText,
  Square,
  SquareTerminal,
  Trash2,
} from 'lucide-react'
import type { DomainFollowUp, RepositoryOffer, WorktreeAction, WorktreeUnoffered } from '@/api/types.gen'

export type MenuAction = WorktreeAction | 'reparent' | 'exec'

export type Section = 'Branch' | 'Run' | 'Open' | 'Remove'

type Runs = 'web' | 'copy' | 'link' | 'later'

// the one list every surface draws, in its order: a key moved here moves in every menu
export const menu: Record<MenuAction, { section: Section; label: string; icon: LucideIcon; runs: Runs }> = {
  fast_forward: { section: 'Branch', label: 'Fast-forward', icon: FastForward, runs: 'later' },
  sync: { section: 'Branch', label: 'Sync', icon: RefreshCw, runs: 'copy' },
  reparent: { section: 'Branch', label: 'Reparent', icon: GitBranch, runs: 'later' },
  exec: { section: 'Run', label: 'Run a command', icon: SquareTerminal, runs: 'later' },
  run_up: { section: 'Run', label: 'Start the profile', icon: Play, runs: 'copy' },
  run_down: { section: 'Run', label: 'Stop the profile', icon: Square, runs: 'copy' },
  logs: { section: 'Run', label: 'View the logs', icon: ScrollText, runs: 'copy' },
  go: { section: 'Open', label: 'Go to the worktree', icon: FolderOpen, runs: 'copy' },
  open_pr: { section: 'Open', label: 'Open the pull request', icon: GitPullRequest, runs: 'link' },
  clean: { section: 'Remove', label: 'Remove', icon: Trash2, runs: 'web' },
}

export const menuActions = Object.keys(menu) as MenuAction[]

const unofferedWords: Record<WorktreeUnoffered, (worktree: string, action: MenuAction) => string> = {
  missing: () => 'Its folder is gone',
  no_branch: () => 'On no branch',
  main_worktree: (worktree, action) =>
    action === 'clean' ? "The main worktree can't be removed" : `${worktree} is the main worktree`,
  base_branch: () => "The base branch's worktree can't be removed",
  not_on_origin: () => 'Not on origin',
  no_pull_request: () => 'No pull request known',
  unrecorded: () => 'Not made by wtm',
  meta_unreadable: () => "Its record doesn't read",
}

export const notInTheWeb = 'Not in the web yet'
const oneAtATime = 'One worktree at a time'

export type Entry = { action: MenuAction; followUp: DomainFollowUp } | { action: MenuAction; disabled: string }

export function entryOf(worktree: string, offers: RepositoryOffer[], action: MenuAction): Entry {
  const offer = offers.find((one) => one.action === action)
  if (menu[action].runs === 'later' || !offer) return { action, disabled: notInTheWeb }
  if (offer.follow_up) return { action, followUp: offer.follow_up }
  return { action, disabled: offer.refused ? unofferedWords[offer.refused](worktree, action) : notInTheWeb }
}

export type Subject = { subject: string; offers: RepositoryOffer[] }

export const menuName = (rows: Subject[]) =>
  rows.length === 1 ? (rows[0]?.subject ?? '') : `${rows.length} selected worktrees`

// on several worktrees, only what the web runs acts on them, on those that offer it; the rest names one worktree
export function selectionEntryOf(rows: Subject[], action: MenuAction): Entry {
  const [only] = rows
  if (rows.length === 1 && only) return entryOf(only.subject, only.offers, action)
  if (menu[action].runs === 'later') return { action, disabled: notInTheWeb }
  if (menu[action].runs !== 'web') return { action, disabled: oneAtATime }

  const offered = rows.flatMap(({ subject, offers }) => {
    const entry = entryOf(subject, offers, action)
    return 'followUp' in entry ? [entry.followUp] : []
  })
  const [first] = offered
  if (!first) return { action, disabled: 'None of the selected worktrees can' }
  return { action, followUp: { ...first, answers: [{ path: 'worktree', values: offered.flatMap(subjectsOf) }] } }
}

export const leftOf = (rows: Subject[], action: MenuAction) =>
  rows.flatMap(({ subject, offers }) => {
    const entry = entryOf(subject, offers, action)
    return 'disabled' in entry ? [{ worktree: subject, why: entry.disabled }] : []
  })

// the live read's actions, but the pull request's from GitHub's read once it landed: the live read never asks
// GitHub, and GitHub's read is not read again on a change, so it says nothing else
export const withPullRequest = (live: RepositoryOffer[], asked: RepositoryOffer[] | undefined) =>
  live.map((offer) => (offer.action === 'open_pr' ? (asked?.find((one) => one.action === 'open_pr') ?? offer) : offer))

export const subjectsOf = (followUp: DomainFollowUp) =>
  followUp.answers?.find((answer) => answer.path === 'worktree')?.values ?? []
