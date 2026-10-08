// A frame of `wtm ui`, transcribed from a run in the sandbox (100 columns) and
// trimmed of what needs GitHub; one frame per selected worktree.
const LEFT = 40;
export const MIN_COLUMNS = 98;
const PROXY = 'acme.localhost';

const worktrees = [
  { name: 'main', sub: 'parent · ● you are here', running: true },
  { name: 'feat/login', sub: 'from main · base ↑1', running: true },
  { name: 'feat/search', sub: 'from main', running: true },
  { name: 'fix/header', sub: 'from main', running: false },
  { name: 'feat/login-ui', sub: 'from feat/login · base ↑2', running: false },
];

export const SELECTABLE = worktrees.filter((w) => w.running).map((w) => w.name);

const esc = (s) => s.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
// Box-drawing and symbol glyphs render a fraction of a pixel wider than letters,
// enough to break the box edges over 98 columns, so each is pinned to one column.
const pin = (s) => s.replace(/[^\x20-\x7e]/gu, (ch) => `<span class="ch">${ch}</span>`);
const len = (s) => [...s].length;
const span = (cls, text) => `<span class="${cls}">${pin(esc(text))}</span>`;

// A cell is a list of [class, text] segments laid out to an exact width.
const cell = (segments, width) => {
  const used = segments.reduce((n, [, t]) => n + len(t), 0);
  const pad = ' '.repeat(Math.max(width - used, 0));
  return segments.map(([cls, t]) => (cls ? span(cls, t) : pin(esc(t)))).join('') + pad;
};
const spread = (left, right, width) => {
  const gap = width - left.reduce((n, [, t]) => n + len(t), 0) - right.reduce((n, [, t]) => n + len(t), 0);
  return [...left, ['', ' '.repeat(Math.max(gap, 1))], ...right];
};

const listRows = (selected) =>
  worktrees.flatMap((w) => {
    const bar = w.name === selected ? ['g', '▌ '] : ['', '  '];
    const nameCls = w.name === selected ? 'b' : '';
    return [
      spread([['', ' '], bar, [nameCls, w.name]], [['g', 'clean'], ['', ' ']], LEFT),
      spread([['', ' '], bar, ['m', w.sub]], w.running ? [['g', '▶ 2 running'], ['', ' ']] : [], LEFT),
      [],
    ];
  });

const detailRows = (selected, RIGHT) => {
  const slug = selected.replaceAll('/', '-');
  const url = (job) => `http://${job}.${slug}.${PROXY}`;
  return [
    spread([['', '   '], ['b', 'DETAIL'], ['m', '    LOGS']], [], RIGHT),
    [['', ' '], ['g', '━━━━━━━━━━'], ['m', '─'.repeat(RIGHT - 12)], ['', ' ']],
    [],
    spread([['', ' '], ['b', selected]], selected === 'main' ? [['g', '● you are here'], ['', ' ']] : [], RIGHT),
    [['', ' '], ['m', '─'.repeat(RIGHT - 2)], ['', ' ']],
    [],
    [['', ' '], ['g', 'clean'], ['m', ' · active just now']],
    [],
    spread([['', ' '], ['b', 'RUN']], [['m', '2 up'], ['', ' ']], RIGHT),
    [],
    spread([['', '   '], ['g', '● '], ['', 'web  '], ['c', url('web')]], [['m', '15s'], ['', ' ']], RIGHT),
    spread([['', '   '], ['g', '● '], ['', 'api  '], ['c', url('api')]], [['m', '15s'], ['', ' ']], RIGHT),
  ];
};

// Like the real TUI, the frame takes the terminal's width: the detail pane grows.
export const frame = (selected, columns = MIN_COLUMNS) => {
  const total = Math.max(columns, MIN_COLUMNS);
  const RIGHT = total - LEFT - 4;
  const left = [[['', ' '], ['b', 'Worktrees']], [], ...listRows(selected)];
  const right = detailRows(selected, RIGHT);
  const height = Math.max(left.length, right.length) + 1;
  const body = Array.from({ length: height }, (_, i) => `${pin('│')}${cell(left[i] ?? [], LEFT)}${pin('││')}${cell(right[i] ?? [], RIGHT)}${pin('│')}`);
  return [
    cell(spread([['', ' '], ['gb', 'wtm'], ['', '  acme · base main · '], ['g', '●'], ['', ' main']], [['m', 'v0.29.2'], ['', ' ']], total), total),
    cell(spread([['', '  '], ['b', 'Worktrees'], ['m', '    Tree    Services']], [['m', '+ New worktree    ⋯ Actions    5 worktrees'], ['', ' ']], total), total),
    span('g', '━━━━━━━━━━━━━') + span('m', '─'.repeat(total - 13)),
    pin(`╭${'─'.repeat(LEFT)}╮╭${'─'.repeat(RIGHT)}╮`),
    ...body,
    pin(`╰${'─'.repeat(LEFT)}╯╰${'─'.repeat(RIGHT)}╯`),
    cell([['m', ' ↑↓ select · n new · m actions · a bulk · tab view · o output · ? help · q quit']], total),
  ];
};
