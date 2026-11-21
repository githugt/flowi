import { describe, expect, it } from 'vitest'
import { claimedOtherHost, isWebUrl, linkHost } from './openurl'

describe('isWebUrl', () => {
  it.each([
    ['https://example.com/a?b=1', true],
    ['http://localhost:8080/cb', true],
    ['HTTPS://EXAMPLE.COM', true],
    ['file:///etc/passwd', false],
    ['smb://host/share', false],
    ['ms-msdt:/id PCWDiagnostic', false],
    ['vscode://file/x', false],
    ['javascript:alert(1)', false],
    ['/relative/path', false],
    ['not a url', false],
    ['', false],
  ])('Given %s Then %s', (url, want) => {
    expect(isWebUrl(url)).toBe(want)
  })
})

describe('linkHost and claimedOtherHost', () => {
  it('names the host an address goes to, and nothing for a non-web address', () => {
    expect(linkHost('https://Docs.Example.com/a?b=1')).toBe('docs.example.com')
    expect(linkHost('file:///etc/passwd')).toBe('')
  })

  // Before: a note's link text could read one host and open another with nothing to tell the person.
  it.each([
    ['docs.example.com', 'https://evil.test/x', 'docs.example.com'],
    ['see https://example.com/docs', 'https://evil.test', 'example.com'],
    ['docs.example.com', 'https://docs.example.com/a', ''],
    ['docs.example.com', 'https://example.com/a', ''],
    ['example.com', 'https://api.example.com', ''],
    ['www.example.com', 'https://example.com', ''],
    ['Read the docs', 'https://example.com', ''],
    ['v1.2 notes', 'https://example.com', ''],
  ])('Given the text %s and the address %s Then the claimed other host is %s', (text, url, want) => {
    expect(claimedOtherHost(text, url)).toBe(want)
  })
})
