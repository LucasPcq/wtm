// expect: nursery/noJsRestrictedProperties
const { fetch: f } = window
export const get = () => f('/x')
