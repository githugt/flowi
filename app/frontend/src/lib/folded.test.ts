import { describe, expect, it } from 'vitest'
import { parseFolded, toggleFolded } from './folded'

describe('toggleFolded', () => {
  it('folds an open group and opens a folded one', () => {
    expect(toggleFolded([], 'a')).toEqual(['a'])
    expect(toggleFolded(['a', 'b'], 'a')).toEqual(['b'])
  })
})

describe('parseFolded', () => {
  it('reads a stored list', () => {
    expect(parseFolded('["a","b"]')).toEqual(['a', 'b'])
  })
  it('gives an empty list for nothing, for damage, and for the wrong shape', () => {
    expect(parseFolded(null)).toEqual([])
    expect(parseFolded('{oops')).toEqual([])
    expect(parseFolded('{"a":1}')).toEqual([])
    expect(parseFolded('["a",1,null]')).toEqual(['a'])
  })
})
