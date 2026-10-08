/** @typedef {{ kind: 'cmd', text: string } | { kind: 'out', html: string, delay?: number }} Line */

export const cmd = (text) => ({ kind: 'cmd', text });
export const out = (html = '', delay) => ({ kind: 'out', html, delay });

const esc = (s) => s.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;');

/** @param {Line[]} lines */
export const render = (lines) =>
  lines.map((l) => (l.kind === 'cmd'
    ? `<span class="ln cmd" data-text="${esc(l.text)}"><span class="typed">${esc(l.text)}</span></span>`
    : `<span class="ln"${l.delay ? ` data-delay="${l.delay}"` : ''}>${l.html}</span>`)).join('');
