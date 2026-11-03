export type LogQuery = { text: string; matchCase: boolean; regex: boolean }

// what a search looks for, none for no text; a regular expression that does not compile looks for nothing, and says so
export function patternOf({ text, matchCase, regex }: LogQuery): RegExp | 'invalid' | undefined {
  if (text === '') return undefined
  const source = regex ? text : text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  try {
    return new RegExp(source, matchCase ? 'g' : 'gi')
  } catch {
    return 'invalid'
  }
}

// the stretches of text pattern matches, as [start, end); an empty match counts for nothing
export function rangesIn(text: string, pattern: RegExp): [number, number][] {
  return [...text.matchAll(new RegExp(pattern.source, pattern.flags))]
    .filter((match) => match[0] !== '')
    .map((match) => [match.index, match.index + match[0].length])
}

export const matchingLines = (lines: { text: string }[], pattern: RegExp) =>
  lines.flatMap((line, at) => (rangesIn(line.text, pattern).length > 0 ? [at] : []))

// a job's output has no level: an error is a line whose text says so
export const errorPattern = /error|fail|panic|exception/i

export function latestError(lines: { text: string }[]): number | undefined {
  const at = lines.findLastIndex((line) => errorPattern.test(line.text))
  return at < 0 ? undefined : at
}

// HH:MM:SS of an instant, in the browser's time zone
export function clock(at: string | undefined): string {
  if (!at) return ''
  const time = new Date(at)
  return [time.getHours(), time.getMinutes(), time.getSeconds()].map((part) => String(part).padStart(2, '0')).join(':')
}

// the first line printed at or after time (HH:MM or HH:MM:SS, the day of the first line held); older when time comes
// before every line held, none when it comes after them
export function lineAt(lines: { at?: string }[], time: string): number | 'older' | undefined {
  const first = lines.find((line) => line.at)?.at
  if (!first) return undefined
  const wanted = instantOn(first, time)
  if (wanted < Date.parse(first)) return 'older'
  const at = lines.findIndex((line) => line.at !== undefined && Date.parse(line.at) >= wanted)
  return at < 0 ? undefined : at
}

// time (HH:MM or HH:MM:SS) on the day of day, in the browser's time zone, as milliseconds since the epoch
export function instantOn(day: string, time: string): number {
  const [hours = 0, minutes = 0, seconds = 0] = time.split(':').map(Number)
  const at = new Date(day)
  at.setHours(hours, minutes, seconds, 0)
  return at.getTime()
}

export type ShownRow = { line: number } | { hidden: number } | { since: number }

// the rows of only the lines matching: each match, the lines hidden before it, and those printed since the last; at is
// where each match is among every line (its index, or its number in the whole log), of count lines
export function onlyMatching(at: number[], count: number): ShownRow[] {
  const rows: ShownRow[] = []
  let after = 0
  at.forEach((position, line) => {
    if (position > after) rows.push({ hidden: position - after })
    rows.push({ line })
    after = position + 1
  })
  if (count > after) rows.push({ since: count - after })
  return rows
}
