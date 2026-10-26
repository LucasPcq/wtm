import type { ListDetail } from '@/api/types.gen'
import { listDetail } from '@/test/fixtures'

// acme, the one repository the component tests' server answers for, with two stacks (feat/billing-ui behind
// feat/billing, feat/search-docs on feat/search) and a worktree with a change (fix/login). A test changes what it
// answers with server.use.
export const repo = { id: 'r1', root: '/acme', common_dir: '/acme/.git' }

export const worktrees: ListDetail[] = [
  listDetail('main', { main: true, parent: '', ahead: null, behind: null, path: '/acme' }),
  listDetail('feat/billing', { ahead: 1 }),
  listDetail('feat/billing-ui', { parent: 'feat/billing', behind: 1, needs_sync: true }),
  listDetail('fix/login', { dirty: true, changes: { changed: 0, untracked: 1 } }),
  listDetail('feat/search'),
  listDetail('feat/search-docs', { parent: 'feat/search' }),
]
