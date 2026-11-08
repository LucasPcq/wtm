import { describe, expect, it } from 'vitest'
import type { DaemonrestartPlan, DaemonstopjobsPlan } from '@/api/types.gen'
import {
  notBackRows,
  repoLine,
  restartRows,
  stopRows,
  stopTotal,
  takenOverRows,
} from '@/features/worktrees/daemon/utils/rows'

const restart: DaemonrestartPlan = {
  running: true,
  to: '0.31.0',
  confirm: true,
  back: [
    {
      root: '/src/acme',
      worktrees: [
        {
          branch: 'main',
          path: '/src/acme',
          jobs: [
            { name: 'postgres', hold: 'own', shared: true, held_by: ['feat/billing', 'fix/login'] },
            { name: 'web', hold: 'own' },
          ],
        },
        { branch: 'feat/billing', path: '/src/acme.trees/feat-billing', jobs: [{ name: 'postgres', hold: 'claim' }] },
      ],
    },
  ],
  taken_over: [
    {
      root: '/src/acme',
      worktrees: [
        { branch: 'fix/login', path: '/src/acme.trees/fix-login', jobs: [{ name: 'mailpit', hold: 'detached' }] },
      ],
    },
  ],
  tasks: [
    { root: '/src/acme', worktrees: [{ branch: 'main', path: '/src/acme', jobs: [{ name: 'migrate', hold: 'own' }] }] },
  ],
  unrecorded: [],
  gone: null,
  unheld: null,
}

describe('a restart', () => {
  it('lists each repository, a shared job on its own run with whose hold goes, a claim on its row', () => {
    expect(restartRows(restart)).toEqual([
      {
        repo: 'acme',
        rows: [
          {
            key: '/src/acme',
            title: 'main',
            lines: ['web', 'postgres · shared'],
            warning: 'feat/billing and fix/login lose it for the cut',
          },
        ],
      },
    ])
  })

  it('says the stacks it takes over and what does not come back', () => {
    expect(takenOverRows(restart)).toEqual([
      {
        key: '/src/acme.trees/fix-login',
        title: "fix/login's stack",
        lines: ['mailpit · started detached: it keeps running'],
      },
    ])
    expect(notBackRows(restart).map((row) => row.lines)).toEqual([['migrate · a task, not run again']])
  })
})

describe('stopping every job', () => {
  const plan: DaemonstopjobsPlan = {
    repos: [
      {
        root: '/src/acme',
        worktrees: [
          {
            worktree: 'main (acme)',
            branch: 'main',
            path: '/src/acme',
            jobs: [
              { name: 'web', hold: 'own', fate: 'stop' },
              { name: 'postgres', hold: 'own', fate: 'stop', shared_in: '/src/acme', held_by: ['feat/billing'] },
            ],
          },
          {
            worktree: 'feat/billing (acme)',
            branch: 'feat/billing',
            path: '/src/acme.trees/feat-billing',
            jobs: [
              { name: 'api', hold: 'own', fate: 'stop' },
              { name: 'mailpit', hold: 'detached', fate: 'stop', stop_command: 'docker compose down' },
              { name: 'worker', hold: 'own', fate: 'clear' },
              { name: 'postgres', hold: 'claim', fate: 'stop', shared_in: '/src/acme' },
            ],
          },
        ],
      },
    ],
  }

  it('lists each worktree, its stack, its crash and its shared services', () => {
    expect(stopRows(plan)[0]?.rows).toEqual([
      { key: '/src/acme', title: 'main', lines: ['web', 'postgres · shared'], warning: 'feat/billing loses it' },
      {
        key: '/src/acme.trees/feat-billing',
        title: 'feat/billing',
        lines: ['api, and its stack: mailpit, through its stop command', 'worker crashed: nothing to stop'],
      },
    ])
  })

  it('counts the jobs that stop, a claim and a crash left out', () => {
    expect(stopTotal(plan)).toBe(4)
  })
})

it('says a repository by its jobs and its stacks', () => {
  expect(
    repoLine({
      root: '/src/acme',
      worktrees: [
        {
          path: '/a',
          jobs: [
            { name: 'web', hold: 'own' },
            { name: 'api', hold: 'own' },
            { name: 'mailpit', hold: 'detached' },
          ],
        },
        {
          path: '/b',
          jobs: [
            { name: 'web', hold: 'own' },
            { name: 'web', hold: 'claim' },
          ],
        },
      ],
    }),
  ).toBe('3 jobs and a stack: web, api, mailpit')
})
