// The storyboard's words and plain-text form. Pure, so what is copied is tested.
import type { ExplainView } from './types'

/** The explanation as plain text, the form `flowi explain` prints: sentences indented two spaces a level, then the problems. */
export function storyText(e: ExplainView): string {
  const lines = e.sentences.map((s) => '  '.repeat(s.indent) + s.text)
  if (e.problems.length) {
    if (lines.length) lines.push('')
    lines.push('Problems:', ...e.problems.map((p) => `  - ${p.text} (step ${p.node})`))
  }
  return lines.join('\n')
}

/** Which tone a row of the storyboard takes from how its step ended ("passed · 120ms", "failed · …"), '' when it has not run. */
export function resultTone(result: string | undefined): 'pass' | 'fail' | '' {
  if (!result) return ''
  if (result.startsWith('passed')) return 'pass'
  if (result.startsWith('failed') || result.startsWith('error')) return 'fail'
  return ''
}

/** The problems of one step, for the marker on its row. */
export function problemsOf(e: ExplainView, node: string): string[] {
  return e.problems.filter((p) => p.node === node).map((p) => p.text)
}
