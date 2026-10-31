import type { DomainWarning, WorktreePullRequestState } from '@/api/types.gen'
import { sentence } from '@/features/worktrees/utils/sentence'

export const isUnavailable = (warning: DomainWarning) => warning.code.endsWith('.prs_unavailable')

export const unavailable = (warnings: DomainWarning[] | undefined) => warnings?.find(isUnavailable)

// an open pull request, the usual case, says nothing beside its number
export const prState: Record<WorktreePullRequestState, string | undefined> = {
  open: undefined,
  merged: 'merged',
  closed: 'closed',
}

// GitHub that could not be asked, as every surface says it: the core names the reason a person fixes outside wtm
export type GitHubReason = 'gh_missing' | 'signed_out' | 'not_on_github'

// next: what a screen that needs GitHub to go on says on its disabled button
export type GitHubWords = { title: string; description: string; next: string; command?: string }

const reasonWords: Record<GitHubReason, GitHubWords> = {
  gh_missing: {
    title: "The GitHub CLI isn't installed",
    description: 'wtm lists pull requests through gh. Install it from cli.github.com, then sign in.',
    next: 'Install gh to list pull requests',
  },
  signed_out: {
    title: "gh isn't signed in to GitHub",
    description: 'Sign in, then open this again.',
    next: 'Sign in to gh to list pull requests',
    command: 'gh auth login',
  },
  not_on_github: {
    title: "origin isn't on GitHub",
    description: 'Pull requests come from GitHub, and origin is elsewhere.',
    next: 'Pull requests need origin on GitHub',
  },
}

// what a command goes without when GitHub could not be asked, by its warning's code
type Unasked =
  | 'list.prs_unavailable'
  | 'tree.prs_unavailable'
  | 'status.prs_unavailable'
  | 'prune.prs_unavailable'
  | 'clean.prs_unavailable'

const unaskedWords: Record<Unasked, string> = {
  'list.prs_unavailable': "Pull requests and their checks aren't shown.",
  'tree.prs_unavailable': "Pull requests and their checks aren't shown.",
  'status.prs_unavailable': "Its pull request and its checks aren't known.",
  'prune.prs_unavailable': "Merged and closed pull requests aren't known, so only branches deleted on origin count.",
  'clean.prs_unavailable': "Open pull requests aren't checked, and merged ones don't count as pushed.",
}

const isReason = (reason: string | undefined): reason is GitHubReason =>
  reason !== undefined && Object.hasOwn(reasonWords, reason)
const isUnasked = (code: string | undefined): code is Unasked => code !== undefined && Object.hasOwn(unaskedWords, code)

export type GitHubFact = { message: string; params?: Record<string, string> | undefined; code?: string }

// a reason the core names has its words, and what the command goes without; any other failure says the core's message
export function githubWords(fact: GitHubFact): GitHubWords & { reason?: GitHubReason } {
  const reason = fact.params?.reason
  if (!isReason(reason))
    return { title: "GitHub couldn't be asked", description: sentence(fact.message), next: "GitHub couldn't be asked" }

  const words = reasonWords[reason]
  return { ...words, reason, ...(isUnasked(fact.code) && { description: unaskedWords[fact.code] }) }
}
