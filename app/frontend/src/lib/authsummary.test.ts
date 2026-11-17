import { describe, expect, it } from 'vitest'
import { authSummary } from './authsummary'

const f = (name: string, value: string) => ({ name, value, secret: false, known: true })

describe('authSummary', () => {
  it('names the header an API key adds, or the query parameter', () => {
    expect(authSummary('scheme:apikey', '', [f('name', 'X-Key'), f('in', 'header')])).toBe('adds the header `X-Key`')
    expect(authSummary('scheme:apikey', '', [f('name', 'k'), f('in', 'query')])).toBe('adds the query parameter `k`')
  })
  it('says what SigV4 signs for', () => {
    expect(authSummary('scheme:aws_sigv4', '', [f('region', 'eu-west-1'), f('service', 's3')])).toContain('eu-west-1 / s3')
  })
  it('knows the plain kinds and says nothing for none', () => {
    expect(authSummary('bearer', '', [])).toContain('Bearer')
    expect(authSummary('none', '', [])).toBe('')
  })
})
