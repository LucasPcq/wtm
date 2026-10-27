import { toast } from 'sonner'

const asText = (line: Promise<string>) => line.then((text) => navigator.clipboard.writeText(text))

// Safari keeps a click's leave to write the clipboard only for a write started in the click: a text still on its way
// is handed over as a promise the clipboard waits on, where the browser takes one, as text where it refuses it
const write = (line: Promise<string>) =>
  typeof ClipboardItem === 'undefined'
    ? asText(line)
    : navigator.clipboard
        .write([new ClipboardItem({ 'text/plain': line.then((text) => new Blob([text], { type: 'text/plain' })) })])
        .catch(() => asText(line))

export function copy(text: string | Promise<string>) {
  const line = Promise.resolve(text)
  Promise.all([line, write(line)]).then(
    ([shown]) => toast.success('Copied', { description: shown }),
    () =>
      line.then(
        (shown) => toast.error("Couldn't copy: the browser refused the clipboard", { description: shown }),
        (error: Error) => toast.error(error.message),
      ),
  )
}
