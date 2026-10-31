// Problems come from the library as "FLW-AREA-NNN: message". People read the
// message; the code is for searching and bug reports, so it goes last and small.

export interface Problem {
  text: string // what to tell the person; `name` marks code
  code: string // FLW-…, or "" when the text had none
}

const CODED = /^(FLW-[A-Z]+-\d+):\s*([\s\S]*)$/
// FLW-TPL-001: undefined `env.base_url` in URL of auth/login (hint)
const UNDEFINED = /^undefined `([^`]+)` in (.+?)(?:\s*\((.+)\))?$/

/** Splits a problem into a plain-language message and its code. */
export function describeProblem(raw: string): Problem {
  const m = CODED.exec(raw.trim())
  if (!m) return { text: raw.trim(), code: '' }
  const [, code, rest] = m
  return { text: code === 'FLW-TPL-001' ? undefinedText(rest) : rest, code }
}

function undefinedText(rest: string): string {
  const m = UNDEFINED.exec(rest)
  if (!m) return rest
  const [, expr, where, hint] = m
  const [root, name] = expr.split('.')
  const what =
    root === 'env' ? `\`${name}\` is not set in the environment` :
    root === 'secret' ? `The secret \`${name}\` is not set in the environment` :
    expr.startsWith('$') ? `\`${expr}\` is not a known function` :
    `\`${expr}\` has no value`
  return `${what} (used in the ${where})${hint ? `: ${hint}` : ''}.`
}

// A run joins several problems into one line; each starts at its code.
const EACH = /\s*(?=FLW-[A-Z]+-\d+:)/

/** Splits text that may hold several coded problems, and describes each. */
export function describeProblems(raw: string): Problem[] {
  return raw.split(EACH).filter((p) => p.trim() !== '').map(describeProblem)
}

/** Cuts text at `backticks` so the code parts can be shown as code. */
export function codeParts(text: string): { text: string; code: boolean }[] {
  return text.split('`').map((t, i) => ({ text: t, code: i % 2 === 1 })).filter((p) => p.text !== '')
}
