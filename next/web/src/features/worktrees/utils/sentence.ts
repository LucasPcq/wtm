// the core's messages are clauses ("its pull request is merged"); one opening on a name keeps its spelling
export const sentence = (message: string) =>
  /^[a-z']+ /.test(message) ? message.charAt(0).toUpperCase() + message.slice(1) : message

// a refusal for its fields says each field's problem, not the request's "invalid"
export const refusalOf = (error: { message: string; fields?: { message: string }[] | undefined }) =>
  error.fields?.length ? error.fields.map((field) => sentence(field.message)).join(' ') : sentence(error.message)
