import { useQuery } from '@tanstack/react-query'
import { githubQuery, readQuery } from '@/api/queries/commands'

export function useRead<C extends 'list' | 'tree'>(repo: string, command: C, enabled = true) {
  return {
    live: useQuery({ ...readQuery(repo, command), enabled }),
    github: useQuery({ ...githubQuery(repo, command), enabled }),
  }
}
