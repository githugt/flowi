import { describe, expect, it } from 'vitest'
import { bodyBytes, copyChoices, copyName, copyText, defaultMode, detailModes, dumpBytes, HEX_LIMIT, hexView, metaChips, parseTree } from './messagedetail'

const json = { direction: 'received', at: '10:00:00', kind: 'json', body: '{"a":1,"b":[2]}' }
const text = { direction: 'received', at: '10:00:00', kind: 'text', body: 'hello' }
const binary = { direction: 'received', at: '10:00:00', kind: 'binary', body: '(binary, 3 bytes)', size: 3, hex: '00000000  00 01 02' }

describe('parseTree', () => {
  it('Given an object or an array When parsed Then it is the value', () => {
    // Before: the message detail was a flat text block, so a JSON body had no tree.
    expect(parseTree('{"a":1}')).toEqual({ value: { a: 1 } })
    expect(parseTree(' [1,2] ')).toEqual({ value: [1, 2] })
  })
  it('Given a lone scalar or broken JSON When parsed Then there is no tree', () => {
    expect(parseTree('42')).toBeNull()
    expect(parseTree('"x"')).toBeNull()
    expect(parseTree('{"a":')).toBeNull()
    expect(parseTree('')).toBeNull()
  })
})

describe('detailModes and defaultMode', () => {
  it('Given a JSON body When asked Then it offers tree, text and hex and opens as a tree', () => {
    expect(detailModes(json)).toEqual(['tree', 'text', 'hex'])
    expect(defaultMode(json)).toBe('tree')
  })
  it('Given a body that claims JSON but does not parse When asked Then there is no tree', () => {
    const bad = { ...json, body: '{"a":' }
    expect(detailModes(bad)).toEqual(['text', 'hex'])
    expect(defaultMode(bad)).toBe('text')
  })
  it('Given plain text When asked Then it opens as text', () => {
    expect(detailModes(text)).toEqual(['text', 'hex'])
    expect(defaultMode(text)).toBe('text')
  })
  it('Given bytes that are not text When asked Then it opens as hex', () => {
    expect(defaultMode(binary)).toBe('hex')
  })
  it('Given a message with no body When asked Then there is nothing to dump', () => {
    expect(detailModes({ body: '', kind: undefined, hex: undefined })).toEqual(['text'])
  })
})

describe('dumpBytes', () => {
  it('Given more than sixteen bytes When dumped Then lines hold sixteen with offset and text', () => {
    const data = new TextEncoder().encode('ABCDEFGHIJKLMNOPQ\n')

    const out = dumpBytes(data).split('\n')

    expect(out).toHaveLength(2)
    expect(out[0]).toBe('00000000  41 42 43 44 45 46 47 48  49 4a 4b 4c 4d 4e 4f 50  |ABCDEFGHIJKLMNOP|')
    expect(out[1].startsWith('00000010  51 0a ')).toBe(true)
    expect(out[1].endsWith('|Q.|')).toBe(true)
  })
  it('Given no bytes When dumped Then it is empty', () => {
    expect(dumpBytes(new Uint8Array())).toBe('')
  })
})

describe('hexView', () => {
  it('Given a text body When dumped Then it is the UTF-8 bytes, not the characters', () => {
    const v = hexView({ body: 'é', kind: 'text' })

    expect(v.total).toBe(2)
    expect(v.text).toContain('c3 a9')
  })
  it('Given a body longer than the limit When dumped Then it says it shows the start', () => {
    const v = hexView({ body: 'x'.repeat(HEX_LIMIT + 10), kind: 'text' })

    expect(v.shown).toBe(HEX_LIMIT)
    expect(v.total).toBe(HEX_LIMIT + 10)
  })
  it('Given a binary message When dumped Then it is the dump the bridge made', () => {
    expect(hexView(binary)).toEqual({ text: '00000000  00 01 02', shown: 3, total: 3 })
  })
})

describe('bodyBytes', () => {
  it('Given a binary message When measured Then it is the bridge size, not the note length', () => {
    // Before: the row measured "(binary, 3 bytes)", a different number from the message.
    expect(bodyBytes(binary)).toBe(3)
    expect(bodyBytes(text)).toBe(5)
  })
})

describe('metaChips', () => {
  it('Given a Kafka message When listed Then partition and offset show with key and type', () => {
    expect(metaChips({ ...text, key: 'k1', type: 'order.paid', partition: 2, offset: 10345 })).toEqual([
      { label: 'Type', value: 'order.paid' },
      { label: 'Key', value: 'k1' },
      { label: 'Partition', value: '2' },
      { label: 'Offset', value: '10345' },
    ])
  })
  it('Given a JetStream message When listed Then the sequence shows', () => {
    expect(metaChips({ ...text, sequence: 7 })).toEqual([{ label: 'Sequence', value: '7' }])
  })
  it('Given a message with none of them When listed Then there are no chips', () => {
    expect(metaChips(text)).toEqual([])
  })
})

describe('copyChoices and copyText', () => {
  it('Given a JSON message When asked Then it can be copied indented', () => {
    expect(copyChoices(json).map((c) => c.value)).toEqual(['body', 'pretty', 'message', 'hex'])
    expect(copyText(json, 'pretty')).toBe('{\n  "a": 1,\n  "b": [\n    2\n  ]\n}')
  })
  it('Given a text message When asked Then there is no indented form', () => {
    expect(copyChoices(text).map((c) => c.value)).toEqual(['body', 'message', 'hex'])
    expect(copyText(text, 'pretty')).toBe('hello')
  })
  it('Given a message When copied whole Then it is JSON without the dump', () => {
    const out = JSON.parse(copyText(binary, 'message'))

    expect(out.body).toBe('(binary, 3 bytes)')
    expect(out.hex).toBeUndefined()
  })
  it('Given a text message When copied as hex Then the whole body is dumped, past the view limit', () => {
    const big = { ...text, body: 'y'.repeat(HEX_LIMIT + 16) }

    expect(copyText(big, 'hex').split('\n')).toHaveLength((HEX_LIMIT + 16) / 16)
  })
  it('Given each form When named Then the notice reads well', () => {
    expect(copyName('hex')).toBe('the hex dump')
  })
})
