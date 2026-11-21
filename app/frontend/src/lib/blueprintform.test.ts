import { describe, expect, it } from 'vitest'
import { missingParams } from './blueprintform'

describe('missingParams', () => {
  const params = [
    { name: 'target', default: '' },
    { name: 'retries', default: '3' },
    { name: 'note', default: '' },
  ]

  it('lists the parameters with no default and no value', () => {
    expect(missingParams(params, { target: '', retries: '3', note: '  ' })).toEqual(['target', 'note'])
  })
  it('is empty when each is given or has a default', () => {
    expect(missingParams(params, { target: 'login', retries: '', note: 'x' })).toEqual([])
  })
})
