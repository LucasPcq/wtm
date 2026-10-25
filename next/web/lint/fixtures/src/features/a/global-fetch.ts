// expect: nursery/noJsRestrictedProperties
export const get = () => globalThis.fetch('/api/x')
