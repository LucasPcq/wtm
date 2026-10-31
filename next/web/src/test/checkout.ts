import { HttpResponse, http } from 'msw'
import type { DomainAnswered, DomainField, DomainOption, WebCall, WorktreePullRequest } from '@/api/types.gen'
import { envelope, followUp } from '@/test/fixtures'
import { line, streamed } from '@/test/stream'

// GitHub as the core reads it for acme: 5 open pull requests (2 mine, 1 asking my review), a closed one and a merged
// one found by number only
const pr = (over: Partial<WorktreePullRequest> & { number: number; branch: string }): WorktreePullRequest => ({
  state: 'open',
  url: `https://github.com/acme/acme/pull/${over.number}`,
  base: 'main',
  ...over,
})

export const pullRequests = [
  pr({ number: 231, branch: 'fix/rate-limit', title: 'Rate-limit the login endpoint', author: 'dana' }),
  pr({
    number: 229,
    branch: 'feat/search-typo',
    title: 'Search: typo tolerance',
    author: 'lucas',
    base: 'feat/search',
    draft: true,
  }),
  pr({ number: 227, branch: 'dark-mode', title: 'Dark mode tokens', author: 'eve', fork: true }),
  pr({ number: 219, branch: 'fix/login-redirect', title: 'Login redirect after SSO', author: 'lucas', draft: true }),
  pr({ number: 214, branch: 'feat/billing', title: 'Billing: invoice totals', author: 'dana' }),
  pr({ number: 205, branch: 'feat/old-onboarding', title: 'Old onboarding flow', author: 'lucas', state: 'closed' }),
  pr({ number: 198, branch: 'feat/search-v1', title: 'Search: first pass', author: 'dana', state: 'merged' }),
]

const held = ['fix/login-redirect', 'feat/billing']

export type GitHub = {
  open?: WorktreePullRequest[]
  // how far fix/rate-limit, a local branch here, is behind origin; none: only origin has it
  behind?: number
  reason?: 'gh_missing' | 'signed_out' | 'not_on_github'
  calls?: WebCall[]
}

const field = (over: Partial<DomainField> & { kind: DomainField['kind']; path: string }): DomainField =>
  ({ ...over }) as DomainField

const asked = (answers: DomainAnswered[] | undefined, path: string) => answers?.find((a) => a.path === path)?.values[0]

const optionOf = (of: WorktreePullRequest, behind: number | undefined): DomainOption => {
  const option: DomainOption = { value: String(of.number), pull_request: of }
  if (of.fork)
    return {
      ...option,
      tag: 'fork',
      problem: {
        path: 'pr',
        code: 'checkout.fork',
        flag: '<number>',
        params: { number: String(of.number), command: `gh pr checkout ${of.number}` },
        message: `pull request #${of.number} is from a fork`,
      },
    }
  if (held.includes(of.branch))
    return {
      ...option,
      tag: 'checked_out',
      problem: {
        path: 'pr',
        code: 'checkout.checked_out_elsewhere',
        flag: '<number>',
        params: { branch: of.branch, number: String(of.number), path: `/acme.trees/${of.branch}` },
        message: `branch ${of.branch} of pull request #${of.number} is checked out`,
        follow_up: followUp('go', of.branch, { code: 'checkout.jump_in', message: 'jump in' }),
      },
    }
  if (behind && of.branch === 'fix/rate-limit') return { ...option, origin: { state: 'behind', ahead: 0, behind } }
  return option
}

export function checkoutQuestionsOf(answers: DomainAnswered[] = [], github: GitHub = {}) {
  const number = asked(answers, 'pr')
  const open = (github.open ?? pullRequests.filter((p) => p.state === 'open')).filter((p) => {
    if (asked(answers, 'mine') === 'true') return p.author === 'lucas'
    if (asked(answers, 'review') === 'true') return p.number === 231
    return true
  })
  const chosen = pullRequests.find((p) => String(p.number) === number)
  const listed = chosen && !open.includes(chosen) ? [...open, chosen] : open
  const options = listed.map((p) => optionOf(p, github.behind))
  const behind = github.behind !== undefined && chosen?.branch === 'fix/rate-limit'
  const problems = () => {
    if (!number) return [{ path: 'pr', code: 'required', flag: '<number>', message: 'pr is required' }]
    if (!chosen)
      return [
        {
          path: 'pr',
          code: 'not_found',
          flag: '<number>',
          params: { number },
          message: `pr: no pull request #${number}`,
        },
      ]
    if (chosen.number === 198)
      return [
        {
          path: 'pr',
          code: 'checkout.head_missing',
          flag: '<number>',
          params: { branch: chosen.branch, number },
          message: `branch ${chosen.branch} of pull request #198 is neither local nor on origin`,
        },
      ]
    return []
  }

  return {
    fields: [
      field({ kind: 'select', path: 'pr', value: number ?? '', options, ...(number && { origin: 'request' }) }),
      field({
        kind: 'select',
        path: 'from',
        value: chosen?.base ?? '',
        refreshable: true,
        options: [{ value: 'main', base: true }, { value: 'feat/search' }],
        ...(chosen && { origin: 'pull_request', pinned: chosen.base ?? '' }),
      }),
      field({
        kind: 'select',
        path: 'env_from',
        value: 'config',
        origin: 'default',
        options: [{ value: 'config' }, { value: 'example' }],
      }),
      field({ kind: 'select', path: 'isolation', value: 'isolated', origin: 'config', skip: 'nothing_isolated' }),
      field({
        kind: 'confirm',
        path: 'fast_forward',
        value: false,
        no_first: true,
        rememberable: true,
        ...(!behind && { skip: 'not_behind' }),
      }),
      field({ kind: 'confirm', path: 'mine', value: asked(answers, 'mine') === 'true', skip: 'flag_only' }),
      field({ kind: 'confirm', path: 'review', value: asked(answers, 'review') === 'true', skip: 'flag_only' }),
      field({ kind: 'confirm', path: 'no_fetch', value: false, skip: 'flag_only' }),
    ],
    problems: problems(),
  }
}

export function checkoutPlan(call: WebCall, github: GitHub = {}) {
  const chosen = pullRequests.find((p) => String(p.number) === asked(call.answers, 'pr'))
  if (!chosen)
    return HttpResponse.json(
      {
        ...envelope('checkout', []),
        ok: false,
        error: { kind: 'invalid', code: 'request.invalid', message: 'the request is invalid' },
      },
      { status: 400 },
    )

  const local = github.behind !== undefined && chosen.branch === 'fix/rate-limit'
  const forwarded = local && asked(call.answers, 'fast_forward') === 'true'
  return HttpResponse.json({
    ...envelope('checkout', []),
    fingerprint: `c:${chosen.number}`,
    warnings:
      chosen.state === 'open'
        ? []
        : [
            {
              code: 'checkout.pr_not_open',
              message: `pull request #${chosen.number} is ${chosen.state}`,
              params: { number: String(chosen.number), state: chosen.state },
            },
          ],
    plan: {
      action: local ? 'check_out' : 'resume',
      pull_request: chosen,
      from: chosen.base,
      env_from: 'example',
      isolation: 'isolated',
      path: `/acme.trees/${chosen.branch}`,
      phases: ['worktree', 'env', 'meta', 'on_create'],
      on_create: ['pnpm install'],
      ...(forwarded && { fast_forward: [chosen.branch] }),
    },
  })
}

const checkoutRun = (call: WebCall) => {
  const branch = pullRequests.find((p) => String(p.number) === asked(call.answers, 'pr'))?.branch ?? ''
  return streamed(
    [
      line({ type: 'unit.started', subject: branch }),
      ...(['worktree', 'env', 'meta', 'on_create'] as const).flatMap((phase) => [
        line({ type: 'phase.started', subject: branch, phase }),
        line({ type: 'phase.finished', subject: branch, phase, failed: false }),
      ]),
      line({ type: 'unit.finished', subject: branch, status: 'done' }),
      `event: envelope\ndata: ${JSON.stringify({
        ...envelope('checkout', []),
        items: [{ subject: branch, status: 'done', detail: { path: `/acme.trees/${branch}` } }],
        follow_ups: [followUp('go', branch, { code: 'checkout.go', message: 'jump in' })],
      })}\n\n`,
    ].join(''),
  )
}

export const answeringCheckout = (github: GitHub = {}) => [
  http.post('/api/repos/r1/commands/checkout/questions', async ({ request }) => {
    const call = (await request.json()) as WebCall
    github.calls?.push(call)
    if (github.reason)
      return HttpResponse.json(
        {
          ...envelope('checkout', []),
          ok: false,
          error: {
            kind: 'precondition',
            code: 'checkout.github_unavailable',
            message: 'pull requests unavailable',
            params: { reason: github.reason },
          },
        },
        { status: 412 },
      )
    return HttpResponse.json(checkoutQuestionsOf(call.answers, github))
  }),
  http.post('/api/repos/r1/commands/checkout', async ({ request }) => {
    const call = (await request.json()) as WebCall
    github.calls?.push(call)
    return call.dry_run ? checkoutPlan(call, github) : checkoutRun(call)
  }),
]
