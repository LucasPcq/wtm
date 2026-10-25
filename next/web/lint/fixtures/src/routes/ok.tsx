// expect: none
import type { QueryClient } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { Page } from '@/features/a/page'
export type Q = QueryClient
export const Route = createFileRoute('/')({ component: Page })
