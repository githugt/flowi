import { describe, expect, it } from 'vitest'
import { maskField, maskValue } from './masked'

describe('maskValue', () => {
  // Before: the Cookies tab showed every cookie value in clear text.
  it('hides the value and its length', () => {
    expect(maskValue('s3cr3t')).toBe('••••••••')
    expect(maskValue('abcdefghijklmnopqrstuvwxyz')).toBe('••••••••••••')
    expect(maskValue('s3cr3t')).not.toContain('s')
  })
  it('keeps an empty value empty', () => {
    expect(maskValue('')).toBe('')
  })
})

describe('maskField', () => {
  it('masks a literal secret and shows a template that points at the vault', () => {
    expect(maskField('hunter2', true, false)).toBe(true)
    expect(maskField('{{secret.token}}', true, false)).toBe(false)
    expect(maskField('Bearer {{secret.token}}', true, false)).toBe(false)
  })
  it('shows a revealed value and never masks an ordinary field', () => {
    expect(maskField('hunter2', true, true)).toBe(false)
    expect(maskField('hunter2', false, false)).toBe(false)
  })
})
