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
