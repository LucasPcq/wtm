// expect: style/noRestrictedImports
import { useLayoutEffect as ule } from 'react'
export function useRun() { ule(() => {}, []) }
