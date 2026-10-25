// expect: nursery/noJsRestrictedProperties
import React from 'react'
export function useRun() { React.useInsertionEffect(() => {}, []) }
