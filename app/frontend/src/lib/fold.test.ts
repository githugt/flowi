import { describe, expect, it } from 'vitest'
import { contains, fold, startsWith } from './fold'
import { filterCommands } from './commands'
import { matches as jsonMatches } from './jsonpath'

describe('fold', () => {
  it('drops accents and case', () => {
    expect(fold('Café Crème')).toBe('cafe creme')
    expect(fold('ÅNGSTRÖM')).toBe('angstrom')
  })
  it('keeps letters that have no accent form', () => {
    expect(fold('Привет')).toBe('привет')
  })
})

describe('contains and startsWith', () => {
  it('find an accented name by its plain spelling and the other way round', () => {
    expect(contains('Café orders', 'cafe')).toBe(true)
    expect(contains('Cafe orders', 'CAFÉ')).toBe(true)
    expect(startsWith('Éclair', 'ec')).toBe(true)
  })
  it('say no when the text is not there', () => {
    expect(contains('Café', 'tea')).toBe(false)
  })
  it('treat an empty query as found', () => {
    expect(contains('anything', '')).toBe(true)
  })
})

describe('searches that use it', () => {
  it('finds a command by its title without the accent', () => {
    const commands = [{ id: 'a', title: 'Résumé a run' }, { id: 'b', title: 'Other' }] as Parameters<typeof filterCommands>[0]

    expect(filterCommands(commands, 'resume').map((c) => c.id)).toEqual(['a'])
  })
  it('finds a JSON value by its plain spelling', () => {
    expect(jsonMatches({ name: 'Zoë' }, 'root', 'zoe')).toBe(true)
  })
})
