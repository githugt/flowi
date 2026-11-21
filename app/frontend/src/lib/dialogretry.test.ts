import { describe, expect, it } from 'vitest'
import { reopened } from './dialogretry'

describe('reopened', () => {
  it('brings a prompt back with the typed text and the reason', () => {
    const prompt = { kind: 'prompt' as const, title: 'Rename', initial: 'old', error: undefined as string | undefined }

    const back = reopened(prompt, 'new name', 'a request named "new name" exists')

    expect(back).toMatchObject({ title: 'Rename', initial: 'new name', error: 'a request named "new name" exists' })
    expect(prompt.initial).toBe('old')
  })
  it('brings a confirm back with the reason and nothing typed', () => {
    const confirm = { kind: 'confirm' as const, title: 'Delete', error: undefined as string | undefined }

    expect(reopened(confirm, '', 'the file is open elsewhere')).toEqual({ kind: 'confirm', title: 'Delete', error: 'the file is open elsewhere' })
  })
})
