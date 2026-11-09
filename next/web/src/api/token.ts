// The token of the server's start arrives in the fragment of the address `wtm-next ui` prints, which the
// browser never sends. localStorage shares it with every tab of the origin: when a restarted server hands a new
// tab a new token, the `storage` event gives it to the tabs already open.
const key = 'wtm.token'

export const token = (): string | null => localStorage.getItem(key)

// the fragment is dropped so that the token never shows in the address bar again
export function takeFragment(): string | undefined {
  const fragment = new URLSearchParams(location.hash.slice(1))
  const fresh = fragment.get('token')
  if (fresh) {
    localStorage.setItem(key, fresh)
    history.replaceState(history.state, '', location.pathname + location.search)
  }
  return fragment.get('repo') ?? undefined
}

export function onTokenChange(listener: () => void): () => void {
  const handle = (event: StorageEvent) => event.key === key && listener()
  addEventListener('storage', handle)
  return () => removeEventListener('storage', handle)
}
