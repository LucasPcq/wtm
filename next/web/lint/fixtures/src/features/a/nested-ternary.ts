// expect: style/noNestedTernary
export const f = (a: number) => (a > 1 ? 'x' : a > 0 ? 'y' : 'z')
