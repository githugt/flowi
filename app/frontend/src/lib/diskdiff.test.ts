import { describe, expect, it } from 'vitest'
import { cell, diffSummary, fieldName } from './diskdiff'
import banner from '../FileBanner.svelte?raw'
import request from '../RequestEditor.svelte?raw'
import flow from '../FlowEditor.svelte?raw'

describe('disk comparison text', () => {
  it('names the field without the root marker', () => {
    expect(fieldName('$.Versions[0].URL')).toBe('Versions[0].URL')
    expect(fieldName('$')).toBe('the whole file')
  })

  it('shows an absent value as nothing', () => {
    expect(cell('')).toBe('(nothing)')
    expect(cell('"PUT"')).toBe('"PUT"')
  })

  it('counts the fields, the ones not listed and the secrets left out', () => {
    expect(diffSummary({ changes: [{ path: '$.a', kind: 'changed', disk: '1', mine: '2' }], more: 0, hidden: 0 })).toBe('1 field differs.')
    expect(diffSummary({ changes: [{ path: '$.a', kind: 'changed', disk: '1', mine: '2' }, { path: '$.b', kind: 'added', disk: '', mine: '3' }], more: 4, hidden: 0 })).toBe('6 fields differ; 4 not listed.')
    expect(diffSummary({ changes: [], more: 0, hidden: 2 })).toBe('No field you can see differs; 2 secret values left out, so a change there is not shown.')
    expect(diffSummary({ changes: [], more: 0, hidden: 0 })).toContain('Nothing differs')
  })
})

describe('the file-changed banner', () => {
  it('offers Compare beside Load from disk and Keep mine, and the editors that can compare pass it', () => {
    // Before: two buttons, Load from disk and Keep mine, with nothing to decide by.
    expect(banner).toContain('Compare')
    expect(banner).toContain('compare?:')
    expect(request).toContain('RequestService.CompareDisk(node)')
    expect(flow).toContain('FlowService.CompareDisk(node)')
  })
})
