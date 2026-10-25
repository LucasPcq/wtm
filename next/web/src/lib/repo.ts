export function repoName(root: string): string {
  return root.split('/').filter(Boolean).at(-1) ?? root
}
