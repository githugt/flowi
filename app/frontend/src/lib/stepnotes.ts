// Where the flow's code view shows the comment of each step: above the line that starts the step.
// Before: the code of a flow built from requests was only ids and paths; nothing in it said what a step was for.

export interface StepNote {
  /** The 1-based line of the step's `- id:` the note is drawn above. */
  line: number
  /** How far the step's dash is indented, in characters, so the note sits with it. */
  indent: number
  text: string
}

const unquote = (s: string) => s.trim().replace(/^(['"])(.*)\1$/, '$2')

/** The notes for the steps in text (the flow body), from comments by step id. A step with no comment gets none. */
export function stepNotes(text: string, comments: Record<string, string | undefined> | undefined): StepNote[] {
  if (!comments) return []
  const out: StepNote[] = []
  text.split('\n').forEach((l, i) => {
    const m = /^(\s*)- id:\s*(.+)$/.exec(l)
    if (!m) return
    const note = comments[unquote(m[2])]
    if (note) out.push({ line: i + 1, indent: m[1].length, text: note })
  })
  return out
}
