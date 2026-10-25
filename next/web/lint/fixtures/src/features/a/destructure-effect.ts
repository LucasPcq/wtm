// expect: nursery/noJsRestrictedProperties
import React from 'react'
const { useEffect } = React
export function useRun() { useEffect(() => {}, []) }
