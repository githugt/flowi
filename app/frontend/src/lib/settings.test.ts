import { describe, expect, it } from 'vitest'
import { labelOf, matches, sectionOf } from './settings'

describe('sectionOf', () => {
  it.each([
    ['appearance.theme', 'Appearance'],
    ['appearance.url.host', 'Appearance'],
    ['appearance.palette_file', 'Advanced'],
    ['telemetry', 'Privacy'],
    ['telemetry_endpoint', 'Privacy'],
    ['announcements_url', 'Privacy'],
    ['announcements_dismissed', 'Advanced'],
    ['timeout', 'Requests'],
    ['http.follow_redirects', 'Network'],
    ['serve.replay_addr', 'Replayer and server'],
    ['logging.level', 'Advanced'],
    ['something.new', 'Advanced'],
  ])('puts %s in %s', (name, section) => {
    expect(sectionOf(name)).toBe(section)
  })
})

describe('labelOf', () => {
  it('reads the last part of the name as words', () => {
    expect(labelOf('http.follow_redirects')).toBe('Follow redirects')
    expect(labelOf('serve.replay_addr')).toBe('Replay address')
    expect(labelOf('tls.ca_files')).toBe('CA files')
  })

  it('prefers a written label', () => {
    expect(labelOf('telemetry')).toBe('Usage reporting')
  })
})

describe('matches', () => {
  it('finds a setting by its label, key or description', () => {
    const f = { name: 'http.max_redirects', description: 'How many redirects to follow.' }
    expect(matches(f, 'Maximum')).toBe(true)
    expect(matches(f, 'http.max')).toBe(true)
    expect(matches(f, 'follow')).toBe(true)
    expect(matches(f, 'proxy')).toBe(false)
    expect(matches(f, '  ')).toBe(true)
  })
})
