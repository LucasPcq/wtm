// sentence is a message of the core, which reads as a clause ("its pull request is merged"), as a sentence; one that
// opens on a name (a branch, a file) keeps it as it is spelled.
export const sentence = (message: string) =>
  /^[a-z]+ /.test(message) ? message.charAt(0).toUpperCase() + message.slice(1) : message
