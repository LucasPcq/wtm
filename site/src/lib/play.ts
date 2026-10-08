const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));
const runs = new WeakMap<Element, number>();

// Types each command and reveals its output line by line. Lines keep their space
// while hidden, so the terminal never changes height.
export const play = async (screen: Element) => {
  if (reduced) return;
  const run = (runs.get(screen) ?? 0) + 1;
  runs.set(screen, run);
  const lines = [...screen.querySelectorAll<HTMLElement>('.ln')];
  lines.forEach((line) => line.classList.add('pending'));
  for (const line of lines) {
    if (runs.get(screen) !== run) return;
    line.classList.remove('pending');
    const typed = line.querySelector<HTMLElement>('.typed');
    if (!typed) {
      await sleep(Number(line.dataset.delay ?? 55));
      continue;
    }
    const text = line.dataset.text ?? '';
    line.classList.add('typing');
    for (let i = 0; i <= text.length; i++) {
      if (runs.get(screen) !== run) return;
      typed.textContent = text.slice(0, i);
      await sleep(14);
    }
    await sleep(300);
    line.classList.remove('typing');
  }
};

export const stop = (screen: Element) => {
  runs.set(screen, (runs.get(screen) ?? 0) + 1);
  screen.querySelectorAll<HTMLElement>('.ln').forEach((line) => {
    line.classList.remove('pending', 'typing');
    const typed = line.querySelector<HTMLElement>('.typed');
    if (typed) typed.textContent = line.dataset.text ?? '';
  });
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
