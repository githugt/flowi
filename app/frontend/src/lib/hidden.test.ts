import { describe, expect, it } from 'vitest'
import { findHidden, revealHidden, stripHidden } from './hidden'

describe('findHidden', () => {
  it('finds a zero-width space and says where it is', () => {
    const got = findHidden('https://api.example.com/​v1')

    expect(got).toEqual([{ index: 24, code: 'U+200B', name: 'zero-width space' }])
  })

  it('finds a non-breaking space, a tab and a byte-order mark', () => {
    expect(findHidden('a b\tc﻿').map((h) => h.name)).toEqual(['non-breaking space', 'tab', 'byte-order mark'])
  })

  it('reports spaces at the start and at the end', () => {
    expect(findHidden('  token ').map((h) => h.name)).toEqual(['2 spaces at the start', '1 space at the end'])
  })

  it('finds nothing in plain text, template braces and non-latin letters', () => {
    expect(findHidden('{{env.base_url}}/users/ñandú/日本')).toEqual([])
  })

  it('does not call an empty or blank text hidden', () => {
    expect(findHidden('')).toEqual([])
  })
})

describe('stripHidden', () => {
  it('removes hidden characters and the spaces at the ends but keeps the inner ones', () => {
    expect(stripHidden(' a​ b c ')).toBe('a bc')
  })
})

describe('revealHidden', () => {
  it('shows each hidden character as its code', () => {
    expect(revealHidden('a​b')).toBe('a⟨U+200B⟩b')
  })
})

describe('joiners that belong in the text', () => {
  // Before: every zero-width joiner and non-joiner was flagged, and "Remove them" changed the word.
  const persian = 'می\u200cخواهم' // "I want", with the non-joiner Persian spells it with
  const family = '👨\u200d👩\u200d👧' // an emoji sequence

  it('leaves a Persian non-joiner and an emoji joiner alone', () => {
    expect(findHidden(persian)).toEqual([])
    expect(findHidden(family)).toEqual([])
    expect(stripHidden(persian)).toBe(persian)
    expect(revealHidden(family)).toBe(family)
  })
  it('still flags a joiner between Latin letters or at an end', () => {
    expect(findHidden('a\u200db').map((h) => h.code)).toEqual(['U+200D'])
    expect(findHidden('\u200cabc').map((h) => h.code)).toEqual(['U+200C'])
    expect(stripHidden('a\u200db')).toBe('ab')
  })
  it('still flags a zero-width space inside Persian text', () => {
    expect(findHidden('می\u200bخواهم').map((h) => h.code)).toEqual(['U+200B'])
  })
})
