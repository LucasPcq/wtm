import { createFileRoute } from '@tanstack/react-router'
import { NoRepository } from '@/features/shell/no-repository'

export const Route = createFileRoute('/_shell/')({ component: NoRepository })
