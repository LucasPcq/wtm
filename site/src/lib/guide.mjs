// The guide's sections are the `##` headings of docs/guide/README.md, each page
// labelled by its link text there, so the index and the sidebar never disagree.
const COLLAPSED_GUIDE_SECTIONS = ['Upgrading'];

const plain = (markdown) => markdown.replaceAll('`', '').replaceAll('*', '').trim();

export const guideSections = (readme) =>
  readme
    .split(/^## /m)
    .slice(1)
    .map((section) => {
      const [heading, ...body] = section.split('\n');
      const items = [...body.join('\n').matchAll(/\[([^\]]+)\]\(([\w.-]+)\.md\)/g)].map(([, text, file]) => ({
        slug: `guide/${file.replaceAll('.', '')}`,
        label: plain(text),
      }));
      const label = plain(heading);
      return { label, collapsed: COLLAPSED_GUIDE_SECTIONS.includes(label), items };
    })
    .filter((section) => section.items.length > 0);
