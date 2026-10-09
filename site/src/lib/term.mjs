/** @typedef {{ kind: 'cmd', text: string } | { kind: 'out', html: string, delay?: number }} Line */

export const cmd = (text) => ({ kind: 'cmd', text });
export const out = (html = '', delay) => ({ kind: 'out', html, delay });

const esc = (s) => s.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;');
const RAIL = '<span class="p">┃</span>';

// wtm frames its human output with a violet rail (internal/surface/cli/render/frame.go) and
// never its JSON; the rail spans the block's first to last non-blank line.
const framed = (text) => /^wtm\s/.test(text) && !/--output json|\|/.test(text);

const railed = (lines) => {
  const result = [];
  let block = [];
  const flush = (frame) => {
    const filled = block.map((l) => l.html.trim() !== '');
    const first = filled.indexOf(true);
    const last = filled.lastIndexOf(true);
    block.forEach((l, i) => result.push(frame && first >= 0 && i >= first && i <= last ? { ...l, html: RAIL + l.html } : l));
    block = [];
  };
  let frame = false;
  for (const line of lines) {
    if (line.kind === 'out') {
      block.push(line);
      continue;
    }
    flush(frame);
    result.push(line);
    frame = framed(line.text);
  }
  flush(frame);
  return result;
};

/** @param {Line[]} lines */
export const render = (lines) =>
  railed(lines).map((l) => (l.kind === 'cmd'
    ? `<span class="ln cmd" data-text="${esc(l.text)}"><span class="typed">${esc(l.text)}</span></span>`
    : `<span class="ln"${l.delay ? ` data-delay="${l.delay}"` : ''}>${l.html}</span>`)).join('');
