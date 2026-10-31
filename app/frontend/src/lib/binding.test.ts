import { describe, expect, it } from 'vitest'
import { bindingValues, bulkSuggestions, completeBulk, mergeBinding, optionNames } from './binding'

const kv = (Key: string, Value: string, Description = '') => ({ Key, Value, Description })
const CREDS = ['username', 'password', 'token']

describe('mergeBinding', () => {
  it('should write each key once, the last row winning', () => {
    const got = mergeBinding([kv('queue', 'a'), kv('queue', 'b')], 'default', [], CREDS)

    expect(got.list).toEqual([kv('queue', 'b')])
  })

  it('should move an option that is a credential to the credentials instead of dropping it', () => {
    const got = mergeBinding([kv('queue', 'q'), kv('username', 'flowi')], 'default', [], CREDS)

    expect(got.list).toEqual([kv('queue', 'q'), kv('username', 'flowi')])
    expect(got.moved).toEqual(['username'])
  })

  it('should not write a credential twice when it is both typed as an option and already a credential', () => {
    const got = mergeBinding([kv('username', 'new')], 'default', [kv('username', 'old')], CREDS)

    expect(got.list.filter((r) => r.Key === 'username')).toEqual([kv('username', 'new')])
  })

  it('should keep the mode of a cloud protocol and write nothing for the default', () => {
    expect(mergeBinding([], 'explicit', [], CREDS).list).toEqual([kv('auth', 'explicit')])
    expect(mergeBinding([], 'default', [], CREDS).list).toEqual([])
  })
})

describe('optionNames', () => {
  it('should list the names of a protocol without its credentials, which have their own tab', () => {
    expect(optionNames(['exchange', 'routing_key', 'queue', 'username', 'password'], ['username', 'password'])).toEqual(['exchange', 'routing_key', 'queue'])
  })
})

describe('bindingValues', () => {
  it.each([
    ['qos', ['0', '1', '2']],
    ['retain', ['true', 'false']],
    ['offset', ['latest', 'earliest']],
    ['read', ['peek', 'lock']],
  ])('should know the values of %s', (key, want) => {
    expect(bindingValues(key)).toEqual(want)
  })

  it('should know no values for a free-text option', () => {
    expect(bindingValues('queue')).toEqual([])
  })
})

describe('bulkSuggestions', () => {
  const names = ['exchange', 'routing_key', 'queue']

  it('should offer the names that start with what is typed on the line, leaving out those already used', () => {
    const text = 'queue: q\nex'

    const got = bulkSuggestions(text, text.length, names)

    expect(got).toEqual({ kind: 'key', items: ['exchange'] })
  })

  it('should offer every unused name on an empty line', () => {
    const text = 'queue: q\n'

    const got = bulkSuggestions(text, text.length, names)

    expect(got).toEqual({ kind: 'key', items: ['exchange', 'routing_key'] })
  })

  it('should offer the values of the option once the colon is typed', () => {
    const text = 'qos: '

    const got = bulkSuggestions(text, text.length, ['qos'])

    expect(got).toEqual({ kind: 'value', items: ['0', '1', '2'] })
  })

  it('should offer nothing for an option whose value is free text', () => {
    const text = 'queue: '

    expect(bulkSuggestions(text, text.length, names)).toBeNull()
  })
})

describe('completeBulk', () => {
  it('should complete a name on its line and put the cursor after the colon', () => {
    const text = 'queue: q\nex'

    const got = completeBulk(text, text.length, 'key', 'exchange')

    expect(got).toEqual({ text: 'queue: q\nexchange: ', caret: 'queue: q\nexchange: '.length })
  })

  it('should complete a value, keeping the rest of the text', () => {
    const text = 'qos: \nqueue: q'

    const got = completeBulk(text, 'qos: '.length, 'value', '1')

    expect(got).toEqual({ text: 'qos: 1\nqueue: q', caret: 'qos: 1'.length })
  })
})
