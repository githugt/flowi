/** How long a revealed secret stays visible before it is hidden again. */
export const REVEAL_MS = 15_000

/**
 * A value as dots, without giving away how long it is: always eight dots for a short value and twelve for a long one.
 * An empty value stays empty, so a cookie with no value does not look like it holds one.
 */
export function maskValue(value: string): string {
  if (!value) return ''
  return '•'.repeat(value.length > 12 ? 12 : 8)
}

/**
 * Whether a secret field shows dots. A value that is a template ({{secret.token}}) is a reference, not the secret,
 * so it is shown; so is anything the person revealed. Before: password-like fields were plain visible inputs.
 */
export function maskField(value: string, secret: boolean, revealed: boolean): boolean {
  return secret && !revealed && !hasTemplate(value)
}

/** Whether the text holds a {{template}}: a reference, which is not itself a secret. */
export function hasTemplate(value: string): boolean {
  return /\{\{[^}]*\}\}/.test(value)
}
