#!/usr/bin/env node
// Mechanical checks of docs/guide against the page shape of the wtm-docs skill.
// Usage: node .claude/skills/wtm-docs/scripts/check-guide.mjs [repo-root]
// Prints one line per finding and exits 1 when there is any.
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const root = process.argv[2] ?? process.cwd();
const dir = join(root, 'docs/guide');
const MAX_LINES = 300;

const readme = readFileSync(join(dir, 'README.md'), 'utf8');
const indexed = new Set([...readme.matchAll(/\]\(([\w.-]+\.md)\)/g)].map((m) => m[1]));
const findings = [];
const report = (file, text) => findings.push(`${file}: ${text}`);

for (const file of readdirSync(dir).filter((f) => f.endsWith('.md') && f !== 'README.md').sort()) {
  const text = readFileSync(join(dir, file), 'utf8');
  const lines = text.split('\n');
  const title = lines[0];

  if (!title.startsWith('# ')) report(file, 'first line is not a `# ` title');
  if (/^# [^`]*:/.test(title)) report(file, `title has a colon subtitle: "${title.slice(2)}"`);
  if (!indexed.has(file)) report(file, 'not listed in docs/guide/README.md, so absent from the sidebar');

  const first = lines.slice(1).find((l) => l.trim() !== '') ?? '';
  if (/^(```|#|\||- |\* |\d+\. |>|<)/.test(first)) report(file, `no lede: the page opens with "${first.slice(0, 40)}"`);

  const tocLinks = lines.filter((l) => /^- \[[^\]]+\]\(#[^)]+\)\s*$/.test(l)).length;
  if (tocLinks >= 3) report(file, `hand-written table of contents (${tocLinks} anchor-only list items)`);

  if (lines.length > MAX_LINES) report(file, `${lines.length} lines: check whether it covers two topics`);

  let fenced = false;
  for (const [i, line] of lines.entries()) {
    if (line.startsWith('```')) fenced = !fenced;
    const next = lines[i + 1] ?? '';
    const prose = (l) => /^[A-Za-z`*(]/.test(l) && !l.startsWith('|');
    if (!fenced && prose(line) && prose(next)) {
      report(file, `line ${i + 1} looks hard-wrapped (two consecutive prose lines)`);
      break;
    }
  }
}

for (const file of indexed) {
  try {
    readFileSync(join(dir, file));
  } catch {
    report('README.md', `links ${file}, which does not exist`);
  }
}

console.log(findings.length ? findings.join('\n') : 'guide: no mechanical finding');
process.exit(findings.length ? 1 : 0);
