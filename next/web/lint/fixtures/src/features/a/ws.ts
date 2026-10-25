// expect: style/noRestrictedGlobals
export const x = () => new WebSocket('ws://x')
