// shellLine is argv as a shell reads it back: each argument quoted only when it must be.
export function shellLine(argv: string[]): string {
  return argv.map((arg) => (/^[\w@%+=:,./-]+$/.test(arg) ? arg : `'${arg.replaceAll("'", `'\\''`)}'`)).join(' ')
}
