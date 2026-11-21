import type { RequestVersion } from './types'

const count = (n: number, one: string) => `${n} ${one}${n === 1 ? '' : 's'}`

// fillNote says what "Fill this request" overwrites in the request that is open, and what it keeps, so the user
// sees the loss before clicking.
// Before: the button replaced the method, URL, query, headers and body with no word about what was lost.
export function fillNote(v: RequestVersion | undefined, wholeFile: boolean): string {
  if (!v) return ''
  const body = v.Body && (v.Body.JSON || v.Body.Text || v.Body.File || v.Body.Form?.length || v.Body.Multipart?.length)
  const lost = [`method ${v.Method || 'GET'}`, v.URL ? 'URL' : '', v.Query?.length ? count(v.Query.length, 'query parameter') : '', v.Headers?.length ? count(v.Headers.length, 'header') : '', body ? 'body' : ''].filter(Boolean)
  const kept = wholeFile
    ? 'Keeps the name and the other versions.'
    : `Keeps ${[v.Assert?.length ? count(v.Assert.length, 'check') : '', v.Extract?.length ? count(v.Extract.length, 'extract') : '', 'outcomes, variables and hooks'].filter(Boolean).join(', ')}.`
  return `Replaces: ${lost.join(', ')}. ${kept} The file changes only when you save.`
}
