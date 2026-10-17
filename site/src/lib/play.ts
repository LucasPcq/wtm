const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));
const runs = new WeakMap<Element, number>();

// Reveals a replayed session line by line, each line whole: a command, then its
// output at the pace wtm printed it. Lines keep their space while hidden, so the
// terminal never changes height.
export const play = async (screen: Element) => {
  if (reduced) return;
  const run = (runs.get(screen) ?? 0) + 1;
  runs.set(screen, run);
  const lines = [...screen.querySelectorAll<HTMLElement>('.ln')];
  lines.forEach((line) => line.classList.add('pending'));
  for (const line of lines) {
    if (runs.get(screen) !== run) return;
    line.classList.remove('pending');
    await sleep(line.classList.contains('cmd') ? 500 : Number(line.dataset.delay ?? 55));
  }
};

const whenSeen = (element: Element, callback: () => void) => {
  const observer = new IntersectionObserver(([entry]) => {
    if (!entry.isIntersecting) return;
    observer.disconnect();
    callback();
  }, { threshold: 0.35 });
  observer.observe(element);
};

export const wireSessions = () => {
  for (const term of document.querySelectorAll<HTMLElement>('.term[data-play]')) {
    const screen = term.querySelector('.screen');
    const replay = term.querySelector<HTMLButtonElement>('.replay');
    if (!screen) continue;
    if (replay && !reduced) {
      replay.hidden = false;
      replay.addEventListener('click', () => play(screen));
    }
    whenSeen(term, () => play(screen));
  }
};

// The hero figure: three lanes braid into one port, the errors dim, the lanes
// split and three worktrees arrive. Played once when seen; replay restarts it.
export const wireCollision = () => {
  const figure = document.querySelector<HTMLElement>('.collision');
  if (!figure) return;
  if (reduced) {
    figure.dataset.phase = '3';
    return;
  }
  let timers: number[] = [];
  const start = () => {
    timers.forEach(clearTimeout);
    figure.dataset.phase = '0';
    timers = [1900, 2700, 3200].map((at, i) => window.setTimeout(() => (figure.dataset.phase = String(i + 1)), at));
  };
  const replay = figure.querySelector<HTMLButtonElement>('.replay');
  if (replay) {
    replay.hidden = false;
    replay.addEventListener('click', start);
  }
  whenSeen(figure, start);
};
