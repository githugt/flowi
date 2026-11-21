import { describe, expect, it } from 'vitest'
import { faultCount, faultsGate, headline, serveUrl } from './replayerhead'

describe('headline', () => {
  it('Given each state Then the top bar says where it stands', () => {
    // Before: the screen showed "listening on" only beside the Stop button at the bottom.
    expect(headline('running', '127.0.0.1:8080')).toEqual({ text: 'Listening on 127.0.0.1:8080', tone: 'ok' })
    expect(headline('running', '')).toEqual({ text: 'Listening', tone: 'ok' })
    expect(headline('starting', '').tone).toBe('busy')
    expect(headline('stopping', '').text).toBe('Stopping…')
    expect(headline('stopped', '')).toEqual({ text: 'Stopped', tone: 'off' })
  })
})

describe('serveUrl', () => {
  it('Given a listen address Then it is the address a client uses', () => {
    expect(serveUrl('127.0.0.1:8080')).toBe('http://127.0.0.1:8080')
    expect(serveUrl(':8080')).toBe('http://127.0.0.1:8080')
    expect(serveUrl('0.0.0.0:9000')).toBe('http://127.0.0.1:9000')
    expect(serveUrl('[::]:9000')).toBe('http://127.0.0.1:9000')
    expect(serveUrl('example.test:80')).toBe('http://example.test:80')
  })
  it('Given nothing Then there is nothing to copy', () => {
    expect(serveUrl('  ')).toBe('')
  })
})

describe('faultsGate', () => {
  it('Given replay only Then faults can act', () => {
    expect(faultsGate({ forward: '' })).toEqual({ enabled: true, reason: '' })
  })
  it('Given forwarding Then they cannot, and it says how to get them', () => {
    const g = faultsGate({ forward: 'pass' })

    expect(g.enabled).toBe(false)
    expect(g.reason).toContain('Replay only')
  })
})

describe('faultCount', () => {
  it('Given some faults set Then they are counted', () => {
    const none = { forward: '', faultErrorPercent: '', faultDropPercent: '0', faultJitter: ' ' }

    expect(faultCount(none)).toBe(0)
    expect(faultCount({ ...none, faultErrorPercent: '20', faultJitter: '300ms' })).toBe(2)
  })
})

describe('ReplayerScreen wiring', () => {
  it('Given the screen Then it has a sticky header, tabs, a reasoned Faults tab and the log beside the form', async () => {
    // Before: a 15-field form with Start at the bottom, and the faults block gone while forwarding.
    const screen = (await import('../ReplayerScreen.svelte?raw')).default as string

    expect(screen).toContain('class="bar"')
    expect(screen).toContain('position: sticky')
    expect(screen).toContain("{ id: 'faults', label: 'Faults'")
    expect(screen).toContain('disabled={running || !gate.enabled}')
    expect(screen).toContain('{gate.reason}')
    expect(screen).toContain('serveUrl(view!.addr)')
    expect(screen).toContain('.running .cols { flex-direction: row')
    expect(screen.indexOf('class="bar"')).toBeLessThan(screen.indexOf('<Tabs'))
  })
})
