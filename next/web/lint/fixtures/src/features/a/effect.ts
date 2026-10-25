// expect: style/noRestrictedImports
import { useEffect } from 'react'
export function useRun() { useEffect(() => {}, []) }
