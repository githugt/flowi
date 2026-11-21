// What the "Copied" toast says about secrets when a code snippet is copied.
// Before: it said "Copied" and nothing else, so a snippet with live secrets left the window with no word on it.

/** Completes "Copied …": the snippet, and whether real secrets are in it or were left out. */
export function snippetCopied(hasSecrets: boolean, excluded: boolean): string {
  if (hasSecrets) return 'the snippet, with your real secrets in it'
  return excluded ? 'the snippet (secrets left out)' : 'the snippet'
}

/** The tooltip of the </> button beside Send: it copies the snippet in the format chosen on the Code tab. */
export function copyCodeLabel(format: string): string {
  return format ? `Copy as ${format}` : 'Copy as code'
}
