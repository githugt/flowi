import { describe, expect, it } from 'vitest'
import { labelOf, matches, modifiedFields, sectionOf } from './settings'

describe('sectionOf', () => {
  it.each([
    ['appearance.theme', 'Appearance'],
    ['appearance.url.host', 'Appearance'],
    ['appearance.palette_file', 'Advanced'],
    ['appearance.send_button', 'Appearance'],
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

  it('names the Send button colour in words, not by its key', () => {
    expect(labelOf('appearance.send_button')).toBe('Send button colour')
    expect(labelOf('appearance.density')).toBe('Spacing')
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

describe('the name of a setting named in a hint', () => {
  it('is the label the Settings screen shows, in the section it is in', () => {
    // Before: hints said "schemas.infer" while the Settings screen showed "Infer".
    expect(labelOf('schemas.infer')).toBe('Learn schemas from sends')
    expect(sectionOf('schemas.infer')).toBe('Requests')
  })
})

describe('modifiedFields', () => {
  it('keeps the settings the person changed', () => {
    const fields = [
      { name: 'timeout', userSet: true },
      { name: 'http.max_redirects', userSet: false },
      { name: 'appearance.theme', userSet: true },
    ]

    // Before: nothing could list the changed settings.
    expect(modifiedFields(fields).map((f) => f.name)).toEqual(['timeout', 'appearance.theme'])
    expect(modifiedFields([])).toEqual([])
  })
})

describe('the Security section', () => {
  // Before: these settings were in Appearance, Requests and Network.
  it.each(['ui.snippet_exclude_secrets', 'datasets.allow_writes', 'http.redirect_credentials', 'http.redirect_keep_headers', 'secrets.host_check', 'secrets.allowed_hosts', 'tls.ca_files'])('holds %s', (name) => {
    expect(sectionOf(name)).toBe('Security')
  })
  it('leaves the other http settings in Network', () => {
    expect(sectionOf('http.follow_redirects')).toBe('Network')
  })
})
