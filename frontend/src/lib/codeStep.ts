// Which top-level step the cursor of the flow's code is in, so the canvas can select it.

const unquote = (s: string) => s.trim().replace(/^(['"])(.*)\1$/, '$2')

/** The id of the top-level step that line (1-based) of the flow text belongs to, or "" when the line is
 *  outside the `nodes:` list (the flow's own settings, a blank before the first step). */
export function stepAtLine(text: string, line: number): string {
  const lines = text.split('\n')
  const start = lines.findIndex((l) => /^nodes:\s*$/.test(l))
  if (start < 0 || line - 1 <= start) return ''
  const first = lines.slice(start + 1).findIndex((l) => /^\s*- /.test(l))
  if (first < 0) return ''
  const itemIndent = lines[start + 1 + first].search(/\S/)
  const isItem = (l: string) => l.search(/\S/) === itemIndent && l.trimStart().startsWith('- ')
  const leavesList = (l: string) => l.trim() !== '' && l.search(/\S/) < itemIndent && !l.trimStart().startsWith('#')

  let i = Math.min(line, lines.length) - 1
  for (; i > start; i--) {
    if (leavesList(lines[i])) return ''
    if (isItem(lines[i])) break
  }
  if (i <= start) return ''
  const own = [lines[i].trimStart().slice(2), ...lines.slice(i + 1).filter((l) => l.search(/\S/) === itemIndent + 2)]
  const idLine = own.find((l) => /^id:/.test(l.trimStart()))
  return idLine ? unquote(idLine.trimStart().slice(3)) : ''
}
