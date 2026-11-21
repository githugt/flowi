import { describe, expect, it } from 'vitest'
import { sizeText } from './bytesize'

describe('sizeText', () => {
  it('writes bytes under 1000', () => expect(sizeText(512)).toBe('512 B'))
  it('writes kilobytes of 1000, as Go does', () => expect(sizeText(1234)).toBe('1.2 KB'))
  it('writes megabytes, never thousands of kilobytes', () => expect(sizeText(5_120_000)).toBe('5.1 MB'))
  it('writes an empty body as 0 B', () => expect(sizeText(0)).toBe('0 B'))
  it('writes nothing for an unknown size', () => expect(sizeText(-1)).toBe(''))
})
