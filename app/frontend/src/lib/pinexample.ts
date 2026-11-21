// Pinning a response as an example writes a file that is committed with the collection. A name that is already taken
// would be written over, so the screen asks first.

/** The words of the dialog that asks before an example is replaced; `name` is what the person typed ("" is the status). */
export function replaceExampleText(name: string, status?: number | string): { title: string; text: string; action: string } {
  const code = /^\d+/.exec(String(status ?? ''))?.[0] // a status line ("200 OK") gives its code
  const shown = name.trim() || code || 'that name'
  return {
    title: 'Replace example',
    text: `An example named ${shown} is already pinned for this request. Pinning again replaces it. If the collection is in git, the old one can be restored from there.`,
    action: 'Replace',
  }
}

/** The words that say what deleting a flowi overlay leaves behind: the flows whose steps name it keep naming it. */
export function overlayDeleteText(name: string, uses: readonly string[]): string {
  const kept = 'Responses kept for it stay in the data folder.'
  if (!uses.length) return `Delete ${name}? ${kept}`
  const list = uses.length <= 5 ? uses.join(', ') : `${uses.slice(0, 5).join(', ')} and ${uses.length - 5} more`
  const who = uses.length === 1 ? `The flow ${list} has a step` : `${uses.length} flows (${list}) have steps`
  return `Delete ${name}? ${who} that lay${uses.length === 1 ? 's' : ''} it over a request, and will fail until another overlay is picked there. ${kept}`
}
