import { describe, expect, it } from 'vitest'
import { hostOf, opensByItself, rememberHost, seenHosts } from './signinhost'

function memory() {
  const m = new Map<string, string>()
  return { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v) }
}

describe('hostOf', () => {
  it('reads the host of a web address and nothing else', () => {
    expect(hostOf('https://login.example.com/authorize?x=1')).toBe('login.example.com')
    expect(hostOf('http://localhost:8080/cb')).toBe('localhost:8080')
    expect(hostOf('javascript:alert(1)')).toBe('')
    expect(hostOf('nonsense')).toBe('')
  })
})

describe('seen hosts', () => {
  it('remembers a host once and reads it back', () => {
    const s = memory()

    rememberHost('login.example.com', s)
    rememberHost('login.example.com', s)

    expect(seenHosts(s)).toEqual(['login.example.com'])
  })
  it('survives a store that holds rubbish or throws', () => {
    expect(seenHosts({ getItem: () => 'not json' })).toEqual([])
    expect(seenHosts({ getItem: () => { throw new Error('blocked') } })).toEqual([])
  })
})

describe('opensByItself', () => {
  it('opens a seen host, and waits for a click on a new one or a bad address', () => {
    expect(opensByItself('https://login.example.com/a', ['login.example.com'])).toBe(true)
    expect(opensByItself('https://evil.example.net/a', ['login.example.com'])).toBe(false)
    expect(opensByItself('file:///etc/passwd', [])).toBe(false)
  })
})
