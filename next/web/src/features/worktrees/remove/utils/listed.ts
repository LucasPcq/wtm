// names as a sentence says them: "a", "a and b", "a, b and c"
export function listed(names: string[]): string {
  if (names.length < 2) return names.join('')
  return `${names.slice(0, -1).join(', ')} and ${names.at(-1)}`
}
