import { expect, test } from 'vitest'
import { changed, dropped, foundIn, hookOf, moved } from '@/features/worktrees/set-up/utils/hooks'

test('a hook reads back from its answer: a command alone, or the JSON of one that runs in a folder or goes on', () => {
  expect(hookOf('pnpm install')).toEqual({ cmd: 'pnpm install' })
  expect(hookOf('{"cmd":"make","cwd":"apps/api","continue_on_error":true}')).toEqual({
    cmd: 'make',
    cwd: 'apps/api',
    continue_on_error: true,
  })
  expect(hookOf('{"command":"make"}')).toEqual({ cmd: '{"command":"make"}' })
  expect(hookOf('{ not json')).toEqual({ cmd: '{ not json' })
})

test('a change to one hook spells it again, back to its command alone once it runs in the root and stops', () => {
  const values = ['pnpm install', 'make']

  expect(changed(values, 1, { cwd: 'apps/api' })).toEqual(['pnpm install', '{"cmd":"make","cwd":"apps/api"}'])
  expect(changed(['{"cmd":"make","cwd":"apps/api"}'], 0, { cwd: '' })).toEqual(['make'])
  expect(changed(['make'], 0, { continue_on_error: true })).toEqual(['{"cmd":"make","continue_on_error":true}'])
  expect(changed(['{"cmd":"make","cwd":"x"}'], 0, { cmd: 'make up' })).toEqual(['{"cmd":"make up","cwd":"x"}'])
})

test('a hook dragged goes where it is dropped, the others keeping their order', () => {
  expect(moved(['a', 'b', 'c'], 0, 2)).toEqual(['b', 'c', 'a'])
  expect(moved(['a', 'b', 'c'], 2, 0)).toEqual(['c', 'a', 'b'])
})

test('the files hooks were found in are listed once each', () => {
  const found = [
    { cmd: 'pnpm install', file: 'pnpm-lock.yaml' },
    { cmd: 'docker compose down', file: 'docker-compose.yml' },
    { cmd: 'pnpm i', file: 'pnpm-lock.yaml' },
  ]

  expect(foundIn(found)).toEqual(['pnpm-lock.yaml', 'docker-compose.yml'])
})

test('a drop moves the hook where it lands, and a drag cancelled, off the list or back to its place changes nothing', () => {
  const values = ['a', 'b', 'c']

  expect(dropped(values, { canceled: false, from: 0, to: 2 })).toEqual(['b', 'c', 'a'])
  expect(dropped(values, { canceled: true, from: 0, to: 2 })).toBeUndefined()
  expect(dropped(values, { canceled: false, from: 0, to: undefined })).toBeUndefined()
  expect(dropped(values, { canceled: false, from: 1, to: 1 })).toBeUndefined()
})
