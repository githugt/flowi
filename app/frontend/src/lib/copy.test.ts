import { describe, expect, it } from 'vitest'
import { copyMessage, copyText, readClipboard } from './copy'

describe('copyText', () => {
  it('writes with the first writer when it works', async () => {
    const seen: string[] = []

    const got = await copyText('hi', [(t) => void seen.push('a' + t), (t) => void seen.push('b' + t)])

    expect(got).toEqual({ ok: true })
    expect(seen).toEqual(['ahi'])
  })

  it('falls back to the next writer when one throws', async () => {
    const seen: string[] = []

    const got = await copyText('hi', [() => { throw new Error('denied') }, (t) => void seen.push(t)])

    expect(got).toEqual({ ok: true })
    expect(seen).toEqual(['hi'])
  })

  it('fails with the last reason when every writer fails, also an async one', async () => {
    const got = await copyText('hi', [() => { throw new Error('first') }, async () => { throw new Error('denied') }])

    expect(got).toEqual({ ok: false, error: 'denied' })
  })

  it('fails when there is no writer', async () => {
    expect((await copyText('hi', [])).ok).toBe(false)
  })
})

describe('copyMessage', () => {
  it('says Copied only for a copy that worked', () => {
    expect(copyMessage('the body', { ok: true })).toEqual({ text: 'Copied the body', level: 'info' })
  })

  it('says why a copy failed and what to do', () => {
    const got = copyMessage('the body', { ok: false, error: 'denied' })

    expect(got.level).toBe('error')
    expect(got.text).toBe('Could not copy the body: denied Select the text and copy it by hand.')
  })
})

describe('readClipboard', () => {
  it('reads with the first reader that works, async or not', async () => {
    expect(await readClipboard([() => { throw new Error('denied') }, async () => 'text'])).toBe('text')
  })
  it('is null when no reader can', async () => {
    expect(await readClipboard([() => { throw new Error('denied') }])).toBeNull()
    expect(await readClipboard([])).toBeNull()
  })
})
