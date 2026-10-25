// @vitest-environment happy-dom
import { beforeEach, expect, test, vi } from 'vitest'
import { onTokenChange, takeFragment, token } from '@/api/token'

beforeEach(() => {
  localStorage.clear()
  history.replaceState(null, '', '/')
})

test('the token of the address is kept and the fragment dropped', () => {
  history.replaceState(null, '', '/#repo=abc&token=t1')

  expect(takeFragment()).toBe('abc')
  expect(token()).toBe('t1')
  expect(location.hash).toBe('')
})

test('an address without a token keeps the one stored', () => {
  localStorage.setItem('wtm.token', 't0')
  history.replaceState(null, '', '/r/abc/worktrees')

  expect(takeFragment()).toBeUndefined()
  expect(token()).toBe('t0')
})

test('another tab storing a token is heard, other keys are not', () => {
  const heard = vi.fn()
  const off = onTokenChange(heard)
  dispatchEvent(new StorageEvent('storage', { key: 'other' }))
  dispatchEvent(new StorageEvent('storage', { key: 'wtm.token' }))
  off()
  dispatchEvent(new StorageEvent('storage', { key: 'wtm.token' }))

  expect(heard).toHaveBeenCalledTimes(1)
})
