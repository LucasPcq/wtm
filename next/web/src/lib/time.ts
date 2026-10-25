const relative = new Intl.RelativeTimeFormat('en', { numeric: 'auto', style: 'short' })

const steps: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 3600],
  ['month', 30 * 24 * 3600],
  ['week', 7 * 24 * 3600],
  ['day', 24 * 3600],
  ['hour', 3600],
  ['minute', 60],
]

export function ago(at: string, now: number): string {
  const seconds = Math.round((now - Date.parse(at)) / 1000)
  for (const [unit, size] of steps) {
    if (seconds >= size) return relative.format(-Math.floor(seconds / size), unit)
  }
  return 'just now'
}
