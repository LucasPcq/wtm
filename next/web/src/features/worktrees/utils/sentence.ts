// the core's messages are clauses ("its pull request is merged"); one opening on a name keeps its spelling
export const sentence = (message: string) =>
  /^[a-z']+ /.test(message) ? message.charAt(0).toUpperCase() + message.slice(1) : message
