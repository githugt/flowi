// Sharing an environment with a collection: the words of the dialog that asks before a file is replaced, and the
// notice after it is written.

/** The question asked when the collection already has a file for this environment. */
export function replaceShareText(env: string, collection: string): { title: string; text: string; action: string } {
  return {
    title: 'Replace shared environment',
    text: `${collection} already has an environment file named ${env}. Replacing it changes what the people who share the collection get. If the collection is in git, the old file can be restored from there.`,
    action: 'Replace',
  }
}

/** The notice after sharing: where it went, that secrets travel as names only, and what else to share. */
export function sharedNotice(env: string, collection: string, notes: readonly string[]): string {
  const base = `Shared ${env} with ${collection}. Its secrets went as names only; each person sets their own values.`
  return notes.length ? `${base} ${notes.join(' ')}` : base
}
