export type LogQuery = { text: string; matchCase: boolean; regex: boolean }

// the whole log is searched by the server, in Go's RE2, which has no lookaround nor backreference
const notRE2 = /\(\?<?[=!]|\\[1-9]|\\k</

// what a search looks for, none for no text; a regular expression that does not compile, or that the server could not
// read, looks for nothing, and says so
export function patternOf({ text, matchCase, regex }: LogQuery): RegExp | 'invalid' | undefined {
  if (text === '') return undefined
  if (regex && notRE2.test(text)) return 'invalid'
  const source = regex ? text : text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  try {
    return new RegExp(source, matchCase ? 'g' : 'gi')
  } catch {
    return 'invalid'
  }
}

// the stretches of text pattern (global, compiled once per search) matches, as [start, end); an empty match counts
// for nothing
export function rangesIn(text: string, pattern: RegExp): [number, number][] {
  const ranges: [number, number][] = []
  pattern.lastIndex = 0
  for (let match = pattern.exec(text); match; match = pattern.exec(text)) {
    if (match[0] === '') pattern.lastIndex++
    else ranges.push([match.index, match.index + match[0].length])
  }
  return ranges
}

// a line's key: its number in the whole log, else its place in the stream, else its place among the lines held
export const lineKey = (line: { n?: number; seq?: number } | undefined, at: number) => line?.n ?? line?.seq ?? at

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

// the first line printed at or after time (HH:MM or HH:MM:SS, on the day of the first line held, else of the last,
// past midnight); older when time comes before every line held, none when it comes after them
export function lineAt(lines: { at?: string }[], time: string): number | 'older' | undefined {
  const first = lines.find((line) => line.at)?.at
  const last = lines.findLast((line) => line.at)?.at
  if (!first || !last) return undefined
  const onFirst = instantOn(first, time)
  const wanted = onFirst < Date.parse(first) ? instantOn(last, time) : onFirst
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
