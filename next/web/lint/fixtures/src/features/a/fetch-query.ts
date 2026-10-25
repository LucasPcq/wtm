// expect: none
import { useQueryClient } from '@tanstack/react-query'
export function useX() { const c = useQueryClient(); return c.fetchQuery }
