import { describe, expect, it } from 'vitest'
import { bindingFields, drift, interfaceFields } from './typesdrift'
import models from '../../bindings/github.com/githugt/flowi/app/internal/bridge/models.js?raw'
import types from './types.ts?raw'

// Interfaces in types.ts that share a name with a Go type but are not it: the diagram's Lane is a drawn lane
// with a position, the bridge's Lane is a row of the inspector.
const NOT_THE_SAME = new Set(['Lane'])

describe('hand-written types against the generated bindings', () => {
  it('reads no field the Go type does not have', () => {
    const found = drift(interfaceFields(types), bindingFields(models))
    for (const name of NOT_THE_SAME) delete found[name]

    // A field here means Go lost or renamed it, or `just bindings` has not run since Go gained it.
    expect(found).toEqual({})
  })
})

describe('bindingFields', () => {
  it('lists the fields of each generated class, optional ones too', () => {
    const text = [
      'export class A {',
      '    constructor($$source = {}) {',
      '        if (!("x" in $$source)) {',
      '            this["x"] = "";',
      '        }',
      '        if (/** @type {any} */(false)) {',
      '            this["y"] = undefined;',
      '        }',
      '    }',
      '}',
      'export class B {',
      '            this["z"] = 0;',
    ].join('\n')

    const got = bindingFields(text)

    expect([...got.get('A')!]).toEqual(['x', 'y'])
    expect([...got.get('B')!]).toEqual(['z'])
  })
})

describe('interfaceFields', () => {
  it('lists the top-level fields of each interface, not those of a nested literal type', () => {
    const text = ['export interface T {', '  id: string', '  note?: string', '  rows: { a: number; b: number }[]', '}', 'export type U = string'].join('\n')

    expect([...interfaceFields(text).get('T')!]).toEqual(['id', 'note', 'rows'])
  })
})

describe('drift', () => {
  it('names the fields a class lacks and skips interfaces with no class', () => {
    const t = new Map([['A', new Set(['x', 'gone'])], ['Local', new Set(['q'])]])
    const m = new Map([['A', new Set(['x'])]])

    expect(drift(t, m)).toEqual({ A: ['gone'] })
  })
})
