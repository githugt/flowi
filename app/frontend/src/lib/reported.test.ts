import { describe, expect, it } from 'vitest'
import { reported } from './reported'

describe('reported', () => {
  it('says ok when the call works, and reports true', async () => {
    const said: string[] = []

    const worked = await reported(Promise.resolve(), (t) => said.push(t), 'Saved')

    expect(worked).toBe(true)
    expect(said).toEqual(['Saved'])
  })

  it('shows the error message when the call fails, and reports false', async () => {
    const said: [string, string | undefined][] = []

    const worked = await reported(Promise.reject(new Error('the vault is locked')), (t, l) => said.push([t, l]), 'Saved')

    expect(worked).toBe(false)
    expect(said).toEqual([['the vault is locked', 'error']])
  })

  it('stays quiet on success when no message is given', async () => {
    const said: string[] = []

    await reported(Promise.resolve(), (t) => said.push(t))

    expect(said).toEqual([])
  })
})
