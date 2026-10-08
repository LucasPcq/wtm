// What both the sync script and astro.config.mjs need to agree on: where the
// repository is, the site's base path, which releases get an archived version,
// and the sidebar each version shows.
import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

export const SITE = dirname(fileURLToPath(import.meta.url));
export const REPO = dirname(SITE);
// The site's own domain once it has one: SITE_URL, BASE, the CNAME file and the
// contact address all follow it. Null publishes to GitHub Pages under /wtm.
export const DOMAIN = process.env.SITE_DOMAIN ?? null;
export const SITE_URL = DOMAIN ? `https://${DOMAIN}` : 'https://lucaspcq.github.io';
export const BASE = process.env.DOCS_BASE ?? (DOMAIN ? '' : '/wtm');
export const DOCS_REF = process.env.DOCS_REF ?? 'main';
export const GENERATED_DIRS = ['guide', 'reference', 'dev'];

// The guide, which archived versions are made of, first shipped in 0.28.
const FIRST_DOCUMENTED_MINOR = [0, 28];

const git = (...args) => execFileSync('git', ['-C', REPO, ...args], { encoding: 'utf8' });

const parse = (tag) => tag.slice(1).split('.').map(Number);
const compare = (a, b) => a[0] - b[0] || a[1] - b[1] || a[2] - b[2];

export const stableTags = git('tag', '--list', 'v*')
  .split('\n')
  .filter((t) => /^v\d+\.\d+\.\d+$/.test(t))
  .sort((a, b) => compare(parse(b), parse(a)));

// One entry per minor, newest first, carrying its latest patch tag.
const minors = [];
for (const tag of stableTags) {
  const [major, minor] = parse(tag);
  if (compare([major, minor, 0], [...FIRST_DOCUMENTED_MINOR, 0]) < 0) continue;
  if (!minors.some((m) => m.major === major && m.minor === minor)) minors.push({ major, minor, tag });
}

const [current, ...archived] = minors;

export const currentLabel = current ? `v${current.major}.${current.minor}` : "Latest";

const archivedSidebar = [
  { label: 'Guide', items: [{ autogenerate: { directory: 'guide' } }] },
  { label: 'Command reference', collapsed: true, items: [{ autogenerate: { directory: 'reference' } }] },
  { slug: 'changelog' },
  { label: 'Contributing', collapsed: true, items: [{ autogenerate: { directory: 'dev' } }] },
];

export const archivedVersions = archived.map(({ major, minor, tag }) => ({
  slug: `${major}-${minor}`,
  label: `v${major}.${minor}`,
  tag,
  sidebar: archivedSidebar,
}));

const slugOf = (file) => `reference/${file.slice(0, -'.md'.length).replaceAll('_', '-')}`;

const commandItem = (command, parentPath) => {
  const label = parentPath ? command.path.slice(parentPath.length + 1) : command.path;
  if (!command.commands?.length) return { slug: slugOf(command.file), label };
  return {
    label,
    collapsed: true,
    items: [{ slug: slugOf(command.file), label: 'Overview' }, ...command.commands.map((c) => commandItem(c, command.path))],
  };
};

// The command reference follows the groups of `wtm --help`, read from the
// index tools/gendocs writes beside the pages.
const referenceItems = () => {
  const index = join(REPO, 'docs/commands.json');
  if (!existsSync(index)) return [{ autogenerate: { directory: 'reference' } }];
  const { groups } = JSON.parse(readFileSync(index, 'utf8'));
  return [
    { slug: 'reference/wtm', label: 'wtm' },
    ...groups.map((g) => ({ label: g.title, collapsed: true, items: g.commands.map((c) => commandItem(c)) })),
  ];
};

export const currentSidebar = [
  { label: 'Guide', items: [{ autogenerate: { directory: 'guide' } }] },
  { label: 'Command reference', collapsed: true, items: referenceItems() },
  { slug: 'changelog' },
  { label: 'Contributing', collapsed: true, items: [{ autogenerate: { directory: 'dev' } }] },
];
