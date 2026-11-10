import type { WebPlace } from '@/api/types.gen'

export type PlaceAnswer = { ok: boolean; line: string; primary?: string }

// what the server found at the path, and the one thing to do with it: a Record over the states the schema publishes
export const placeAnswer: Record<WebPlace['state'], (place: WebPlace) => PlaceAnswer> = {
  missing: ({ path }) => ({ ok: false, line: `Nothing is there: ${path} doesn't exist.` }),
  not_a_repository: ({ path }) => ({ ok: false, line: `Not a git repository: there is no .git in ${path}.` }),
  not_a_folder: ({ path }) => ({ ok: false, line: `${path} is a file, not a folder.` }),
  not_set_up: ({ name }) => ({ ok: true, line: 'A git repository, not set up yet.', primary: `Set up ${name}` }),
  set_up: ({ name }) => ({ ok: true, line: `${name} is set up already: it's added as it is.`, primary: `Add ${name}` }),
  known: ({ name }) => ({ ok: true, line: `${name} is already here.`, primary: `Open ${name}` }),
}
