import { createFileRoute, redirect } from '@tanstack/react-router'

export const Route = createFileRoute('/_shell/r/$repo/')({
  beforeLoad: ({ params }) => {
    throw redirect({ to: '/r/$repo/worktrees', params })
  },
})
