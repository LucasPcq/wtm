// expect: nursery/noJsRestrictedProperties
export const open = () => new self.WebSocket('ws://x')
