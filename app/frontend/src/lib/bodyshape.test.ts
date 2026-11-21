import { describe, expect, it } from 'vitest'
import { pickShape, shapeIsFormat } from './bodyshape'

const body = (o: Partial<Parameters<typeof pickShape>[1]>) => ({ isDataset: false, hasTable: false, bodyKind: 'json', hasHtml: false, ...o })

describe('pickShape', () => {
  it('should read a plain response as a tree and a dataset as rows', () => {
    expect(pickShape('', body({}))).toBe('tree')
    expect(pickShape('', body({ isDataset: true, hasTable: true }))).toBe('rows')
  })

  it('should preview a page and give pictures, PDFs and binary bodies the media viewer', () => {
    // Before: these were drawn as plain text.
    expect(pickShape('', body({ bodyKind: 'html', hasHtml: true }))).toBe('preview')
    expect(pickShape('', body({ bodyKind: 'image' }))).toBe('media')
    expect(pickShape('', body({ bodyKind: 'pdf' }))).toBe('media')
    expect(pickShape('', body({ bodyKind: 'binary' }))).toBe('media')
  })

  it('should keep a pick that still applies and drop one that does not', () => {
    expect(pickShape('raw', body({ bodyKind: 'image' }))).toBe('raw')
    expect(pickShape('preview', body({ bodyKind: 'image' }))).toBe('media')
    expect(pickShape('media', body({}))).toBe('tree')
    expect(pickShape('rows', body({}))).toBe('tree')
  })
})

describe('shapeIsFormat', () => {
  it('should tell Go only the text formats', () => {
    expect(['json', 'text', 'raw'].every(shapeIsFormat)).toBe(true)
    expect(['tree', 'status', 'rows', 'preview', 'media'].some(shapeIsFormat)).toBe(false)
  })
})
