// expect: nursery/noJsRestrictedProperties, complexity/useLiteralKeys
import React from 'react'
export function useRun() { React['useEffect'](() => {}, []) }
