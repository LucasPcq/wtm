import { expect, onTestFinished, test, vi } from 'vitest'
import { copy } from '@/features/worktrees/utils/copy'

type Written = { item?: Record<string, Promise<Blob>> | undefined; text?: string }

// a clipboard that takes items or refuses them, and a ClipboardItem that keeps what it was given
function stubClipboard(takesItems: boolean) {
  const written: Written[] = []
  const clipboard = Object.getOwnPropertyDescriptor(navigator, 'clipboard')
  const item = Object.getOwnPropertyDescriptor(globalThis, 'ClipboardItem')
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: {
      write: async ([given]: { items: Record<string, Promise<Blob>> }[]) => {
        if (!takesItems) throw new Error('refused')
        written.push({ item: given?.items })
      },
      writeText: async (text: string) => {
        written.push({ text })
      },
    },
  })
  Object.defineProperty(globalThis, 'ClipboardItem', {
    configurable: true,
    writable: true,
    value: class {
      readonly items: Record<string, Promise<Blob>>
      constructor(items: Record<string, Promise<Blob>>) {
        this.items = items
      }
    },
  })
  onTestFinished(() => {
    if (clipboard) Object.defineProperty(navigator, 'clipboard', clipboard)
    else Reflect.deleteProperty(navigator, 'clipboard')
    if (item) Object.defineProperty(globalThis, 'ClipboardItem', item)
    else Reflect.deleteProperty(globalThis, 'ClipboardItem')
  })
  return written
}

test('a line still on its way is written as an item the clipboard waits on, in the click', async () => {
  const written = stubClipboard(true)
  let spell = (_line: string) => {}

  copy(new Promise<string>((resolve) => (spell = resolve)))

  expect(written).toHaveLength(1)
  spell('wtm sync feat/a')
  await expect(written[0]?.item?.['text/plain']?.then((blob) => blob.text())).resolves.toBe('wtm sync feat/a')
})

test('a clipboard that refuses an item is written the line as text', async () => {
  const written = stubClipboard(false)

  copy(Promise.resolve('wtm sync feat/a'))

  await vi.waitFor(() => expect(written).toEqual([{ text: 'wtm sync feat/a' }]))
})
