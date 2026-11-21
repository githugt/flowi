/** Whether a new vault passphrase, typed twice, can be used. */
export type NewPassphrase = 'ok' | 'empty' | 'mismatch'

/**
 * Checks the two entries of a new passphrase. Before: the vault asked once, with the same box as for unlocking, so a
 * typo locked the secrets for good (the passphrase cannot be recovered).
 */
export function checkNewPassphrase(first: string, second: string): NewPassphrase {
  if (first.trim() === '') return 'empty'
  return first === second ? 'ok' : 'mismatch'
}

/** The text that tells the person what is wrong with a new passphrase, or '' when it is fine. */
export function newPassphraseProblem(result: NewPassphrase): string {
  switch (result) {
    case 'empty':
      return 'The passphrase cannot be empty. Choose one and enter it again.'
    case 'mismatch':
      return 'The two passphrases were not the same. Enter them again.'
    default:
      return ''
  }
}
