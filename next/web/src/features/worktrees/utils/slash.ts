// a ref callback: / focuses input from anywhere on the page but another field
export function focusOnSlash(input: HTMLInputElement | null) {
  if (!input) return

  const onKey = (e: KeyboardEvent) => {
    const typing = e.target instanceof HTMLElement && e.target.closest('input, textarea, select, [contenteditable]')
    if (e.key !== '/' || typing || e.metaKey || e.ctrlKey || e.altKey) return
    e.preventDefault()
    input.focus()
  }
  document.addEventListener('keydown', onKey)
  return () => document.removeEventListener('keydown', onKey)
}
