import { InlineStyleAnnotation } from '@astrojs/starlight/expressive-code';
import { createHighlighter } from 'shiki';

const PROMPT = '$ ';

// Shiki's `console` grammar leaves the command after `$ ` uncoloured (the bash grammar only finds a
// command name at the start of a line) and colours output at random. This replaces its annotations:
// each command is highlighted alone as `bash`, so it reads exactly like a `bash` block, and output stays plain.
export function pluginConsoleHighlight() {
  const highlighter = createHighlighter({ langs: ['bash'], themes: [] });
  return {
    name: 'wtm-console-highlight',
    hooks: {
      performSyntaxAnalysis: async ({ codeBlock, styleVariants }) => {
        if (codeBlock.language !== 'console') return;
        const shiki = await highlighter;
        const lines = codeBlock.getLines();
        lines.forEach((line) => {
          line.getAnnotations()
            .filter((annotation) => annotation instanceof InlineStyleAnnotation)
            .forEach((annotation) => line.deleteAnnotation(annotation));
        });
        for (const [styleVariantIndex, { theme }] of styleVariants.entries()) {
          if (!shiki.getLoadedThemes().includes(theme.name)) await shiki.loadTheme({ ...theme, settings: theme.settings ?? [] });
          lines.filter((line) => line.text.startsWith(PROMPT)).forEach((line) => {
            const [tokens] = shiki.codeToTokensBase(line.text.slice(PROMPT.length), { lang: 'bash', theme: theme.name });
            tokens.reduce((columnStart, token) => {
              const columnEnd = columnStart + token.content.length;
              line.addAnnotation(new InlineStyleAnnotation({
                styleVariantIndex,
                color: token.color || theme.fg,
                italic: (token.fontStyle & 1) === 1,
                bold: (token.fontStyle & 2) === 2,
                inlineRange: { columnStart, columnEnd },
                renderPhase: 'earliest',
              }));
              return columnEnd;
            }, PROMPT.length);
          });
        }
      },
    },
  };
}
