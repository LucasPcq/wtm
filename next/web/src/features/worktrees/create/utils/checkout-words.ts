import type {
  CheckoutAction,
  CheckoutRequest,
  DomainError,
  DomainFieldError,
  DomainOption,
  DomainOptionTag,
  DomainWarning,
  WorktreePullRequest,
} from '@/api/types.gen'
import type { FieldWords } from '@/components/command-form/command-fields'
import type { StepKey } from '@/features/worktrees/create/utils/steps'
import { createWords } from '@/features/worktrees/create/utils/words'
import { githubWords, prState } from '@/features/worktrees/utils/github'
import { sentence } from '@/features/worktrees/utils/sentence'
import { plural } from '@/lib/plural'

// checkout's questions in Create's dialog: the filters and no_fetch are answered by the screen, never asked
export type CheckoutQuestion = Exclude<
  CheckoutRequest[number]['path'],
  'remember' | 'ask' | 'mine' | 'review' | 'no_fetch'
>

export const checkoutSteps: Record<CheckoutQuestion, StepKey> = {
  pr: 'branch',
  from: 'branch',
  fast_forward: 'branch',
  env_from: 'environment',
  isolation: 'environment',
}

// the help of the fast-forward names the branch and how far behind it is, as the chosen pull request's option says
export const checkoutWords = (chosen: DomainOption | undefined): Record<CheckoutQuestion, FieldWords> => {
  const branch = chosen?.pull_request?.branch
  const behind = chosen?.origin?.behind ?? 0
  return {
    pr: { label: 'Pull request', help: 'a number finds a closed or merged one too.' },
    from: { label: 'Parent', help: 'The branch it syncs onto.' },
    fast_forward: {
      label: 'Source update',
      help:
        branch && behind > 0
          ? `${branch} is ${plural(behind, 'commit')} behind origin.`
          : 'Its branch is behind origin.',
      options: {
        true: { title: 'Update it first', description: 'Fast-forward it to origin, then check it out.' },
        false: { title: 'Keep it as it is', description: 'Check it out where it stands now.' },
      },
    },
    env_from: createWords.env_from,
    isolation: createWords.isolation,
  }
}

export const prFilters = ['all', 'mine', 'review'] as const
export type PrFilter = (typeof prFilters)[number]

export const prFilterLabel: Record<PrFilter, string> = { all: 'All', mine: 'Mine', review: 'To review' }

const actionTag: Record<CheckoutAction, { fetched: string; stale: string }> = {
  check_out: { fetched: 'Existing branch', stale: 'Existing branch' },
  resume: { fetched: 'Fetched from origin', stale: 'From origin, as last fetched' },
}

export const tagOf = (action: CheckoutAction, fetched: boolean) => actionTag[action][fetched ? 'fetched' : 'stale']

// the line under a pull request's title: who, into what, and its state when it has one
const prLine = (pr: WorktreePullRequest) =>
  [pr.author, pr.base && `into ${pr.base}`, pr.draft && 'draft', prState[pr.state]].filter(Boolean).join(' · ')

// a tag that keeps a pull request from being chosen, and what its row says instead
const tagLine: Record<DomainOptionTag, ((option: DomainOption) => string) | null> = {
  fork: (option) => `From a fork: check it out with ${option.problem?.params?.command ?? 'gh pr checkout'}`,
  checked_out: (option) =>
    [`Already in ${option.pull_request?.branch ?? 'a worktree'}`, option.pull_request?.draft && 'draft']
      .filter(Boolean)
      .join(' · '),
  local: null,
  remote: null,
  current: null,
  main: null,
}

export const optionLine = (option: DomainOption) => {
  const refused = option.tag && tagLine[option.tag]
  if (refused) return refused(option)
  return option.pull_request ? prLine(option.pull_request) : ''
}

type PrRefusal =
  | 'checkout.fork'
  | 'checkout.checked_out_elsewhere'
  | 'checkout.path_exists'
  | 'checkout.name_clash'
  | 'checkout.head_missing'
  | 'not_found'

const refusalWords: Record<PrRefusal, (params: Record<string, string>) => string> = {
  'checkout.fork': (p) => `#${p.number} is from a fork: check it out with ${p.command}.`,
  'checkout.checked_out_elsewhere': (p) => `#${p.number} is already checked out in ${p.branch}.`,
  'checkout.path_exists': (p) => `Its folder ${p.path} already exists.`,
  'checkout.name_clash': (p) => `${p.branch} would share its folder name with ${p.other}.`,
  'checkout.head_missing': (p) => `#${p.number}'s branch is gone from origin: there is nothing to check out.`,
  not_found: (p) => `GitHub has no pull request #${p.number}.`,
}

const isRefusal = (code: string): code is PrRefusal => Object.hasOwn(refusalWords, code)

// one the core adds shows the core's message until it is worded here
export const refusalLine = (error: DomainFieldError) =>
  isRefusal(error.code) ? refusalWords[error.code](error.params ?? {}) : sentence(error.message)

type RowWarning = 'checkout.source_diverged' | 'checkout.pr_not_open' | 'checkout.fetch_failed'

const warningWords: Record<RowWarning, (params: Record<string, string>) => string> = {
  'checkout.source_diverged': (p) =>
    `diverged from origin (${p.ahead} ahead, ${p.behind} behind): starts from your branch`,
  'checkout.pr_not_open': (p) => `#${p.number} is ${p.state}: checked out as it was left`,
  'checkout.fetch_failed': () => "couldn't fetch it from origin: where it stands is as last fetched",
}

// a warning of the pull request or its branch goes on its row; any other is the plan's Alert
export const isRowWarning = (warning: DomainWarning): warning is DomainWarning & { code: RowWarning } =>
  Object.hasOwn(warningWords, warning.code)

export const rowWarningLine = (warning: DomainWarning & { code: RowWarning }) =>
  warningWords[warning.code](warning.params ?? {})

// what Next says while the form cannot be asked: GitHub's words, or any other refusal
export const blockedBy = (refusal: DomainError | undefined) =>
  refusal?.code === 'checkout.github_unavailable'
    ? githubWords(refusal).next
    : "Checkout can't start: see the error above"
