import { describe, expect, it } from 'vitest'
import { percentAsShare, runCommand, serveCommand, shellWord } from './cliline'
import type { ReplayerView } from './types'

const blank: ReplayerView['form'] = { addr: '', latency: '', sequence: false, cors: false, forward: '', forwardTo: '', forwardOnly: '', env: '', recordTo: '', recordFolder: '', recordHar: '', faultErrorPercent: '', faultStatus: '', faultDropPercent: '', faultJitter: '' }

describe('shellWord', () => {
  it('leaves a plain word and quotes the rest', () => {
    expect(shellWord('127.0.0.1:8080')).toBe('127.0.0.1:8080')
    expect(shellWord('my file.har')).toBe(`'my file.har'`)
    expect(shellWord(`it's`)).toBe(`'it'\\''s'`)
  })
})

describe('percentAsShare', () => {
  it('turns the form percent into the flag share', () => {
    expect(percentAsShare('20')).toBe('0.2')
    expect(percentAsShare('250')).toBe('1')
    expect(percentAsShare('')).toBe('')
    expect(percentAsShare('0')).toBe('')
    expect(percentAsShare('abc')).toBe('')
  })
})

describe('serveCommand', () => {
  it('is just the replayer for a blank form', () => {
    expect(serveCommand(blank)).toBe('flowi serve --no-triggers --no-schedules')
  })
  it('writes the faults as shares, in the units the flags take', () => {
    const cmd = serveCommand({ ...blank, addr: ':9000', faultErrorPercent: '20', faultStatus: '502', faultJitter: '300ms', sequence: true })

    expect(cmd).toBe('flowi serve --no-triggers --no-schedules --replay-addr :9000 --sequence --fault-error 0.2 --fault-status 502 --fault-jitter 300ms')
  })
  it('writes forwarding and leaves the faults out, which do not apply while forwarding', () => {
    const cmd = serveCommand({ ...blank, forward: 'save', forwardTo: 'https://api.test', recordTo: 'C1', recordHar: 'my s.har', faultErrorPercent: '50' })

    expect(cmd).toContain('--forward save --forward-to https://api.test --record-to C1')
    expect(cmd).toContain(`--record-har 'my s.har'`)
    expect(cmd).not.toContain('--fault-error')
  })
})

describe('runCommand', () => {
  const base = { path: 'user-creation', collection: 'User creation', env: 'staging', inputs: {}, seed: '' }

  it('names the flow, the collection and the environment', () => {
    expect(runCommand(base)).toBe(`flowi run user-creation --collection 'User creation' --env staging`)
  })
  it('leaves out an environment that is not selected', () => {
    expect(runCommand({ ...base, env: '' })).toBe(`flowi run user-creation --collection 'User creation'`)
  })
  it('adds each input in a fixed order, quoting what a shell would split, and keeps an empty one', () => {
    const cmd = runCommand({ ...base, inputs: { name: 'Ada Lovelace', email: 'a@x.test', note: '' } })
    expect(cmd).toBe(`flowi run user-creation --collection 'User creation' --env staging --input email=a@x.test --input 'name=Ada Lovelace' --input note=`)
  })
  it('repeats the last run with its seed', () => {
    expect(runCommand({ ...base, seed: '14951492923856496236' })).toMatch(/ --seed 14951492923856496236$/)
  })
})
