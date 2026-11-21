import { describe, expect, it } from 'vitest'
import { headerWord, sectionTitle } from './sectiontitle'

describe('sectionTitle', () => {
  it('calls the headers of a gRPC request metadata', () => {
    // Before: Headers, with the HTTP hint.
    expect(sectionTitle('grpc', 'Headers')).toBe('Metadata')
    expect(sectionTitle('grpc', 'Message')).toBe('Body')
    expect(sectionTitle('grpc', 'Receive')).toBe('Stream')
  })
  it('leaves every other kind as it is', () => {
    expect(sectionTitle('http', 'Headers')).toBe('Headers')
    expect(sectionTitle('async', 'Message')).toBe('Message')
  })
})

describe('headerWord', () => {
  it('is metadata for gRPC and header otherwise', () => {
    expect(headerWord('grpc')).toBe('metadata')
    expect(headerWord('http')).toBe('header')
  })
})
