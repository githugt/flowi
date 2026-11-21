import { describe, expect, it } from 'vitest'
import { sourceKey } from './methodsource'

describe('sourceKey', () => {
  const base = { Server: 'localhost:50051', Proto: null, Import: null }

  it('is the same for the same source, and treats a missing list as an empty one', () => {
    expect(sourceKey(base)).toBe(sourceKey({ ...base, Proto: [], Import: [] }))
  })

  it('changes with the server, the proto files and the import paths', () => {
    const k = sourceKey(base)
    expect(sourceKey({ ...base, Server: 'localhost:9' })).not.toBe(k)
    expect(sourceKey({ ...base, Proto: ['a.proto'] })).not.toBe(k)
    expect(sourceKey({ ...base, Import: ['.'] })).not.toBe(k)
  })

  it('does not mix a server with a proto path', () => {
    expect(sourceKey({ Server: 'a', Proto: ['b'], Import: null })).not.toBe(sourceKey({ Server: 'ab', Proto: [], Import: null }))
  })
})
