// Finding {#each} blocks with no key, for the list of work left (UI3-E10) and a ratchet that keeps it from growing.
// A list a person edits or reorders needs a key per row, or Svelte reuses the row at the same index and the typed
// text, focus and open state stay on the wrong row.

export interface EachBlock {
  line: number
  /** The text of the opening tag. */
  head: string
  keyed: boolean
  /** The block holds an input bound to the row (bind:), so it is a list that is edited in place. */
  editable: boolean
}

/** Every {#each …} in a Svelte file's text. A key is the (expr) that ends the opening tag. */
export function eachBlocks(text: string): EachBlock[] {
  const lines = text.split('\n')
  const out: EachBlock[] = []
  lines.forEach((l, i) => {
    const at = l.indexOf('{#each ')
    if (at < 0) return
    const head = l.slice(at, l.indexOf('}', at) + 1)
    const keyed = /\)\s*\}$/.test(head)
    let end = i
    while (end < Math.min(i + 12, lines.length - 1) && !lines[end].includes('{/each}')) end++
    const editable = lines.slice(i, end + 1).some((x) => /\bbind:(value|checked|group)=\{\w+\./.test(x))
    out.push({ line: i + 1, head, keyed, editable })
  })
  return out
}
