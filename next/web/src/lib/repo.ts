// repoName is how a repository is called on screen: the last element of its main worktree's path.
export function repoName(root: string): string {
  return root.split('/').filter(Boolean).at(-1) ?? root
}
