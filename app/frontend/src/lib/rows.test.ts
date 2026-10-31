import { describe, expect, it } from 'vitest'
import { dockTabs, openAfterClick } from './card'

describe('openAfterClick', () => {
  it('opens the clicked row', () => {
    expect(openAfterClick(null, 'login', 'inputs', true)).toEqual({ unit: 'login', key: 'inputs' })
  })
  it('closes the row that is open when it is clicked again', () => {
    expect(openAfterClick({ unit: 'login', key: 'inputs' }, 'login', 'inputs', true)).toBeNull()
  })
  it('opens another row of the same card in place of the open one', () => {
    expect(openAfterClick({ unit: 'login', key: 'inputs' }, 'login', 'hooks', true)).toEqual({ unit: 'login', key: 'hooks' })
  })
  it('opens a row of another card in place of the open one', () => {
    expect(openAfterClick({ unit: 'login', key: 'inputs' }, 'verify', 'inputs', true)).toEqual({ unit: 'verify', key: 'inputs' })
  })
  it('opens nothing while viewing: there is no editor to show', () => {
    expect(openAfterClick(null, 'login', 'inputs', false)).toBeNull()
  })
})

describe('dockTabs', () => {
  const none = { input: [], output: [], tests: [], messages: [], diags: [] }
  it('offers Input, Output and Checks for a request', () => {
    expect(dockTabs({ ...none, kind: 'request', input: [1], output: [1], tests: [1] })).toEqual(['input', 'output', 'checks'])
  })
  it('offers Messages and Checks for a message request, not Input and Output', () => {
    expect(dockTabs({ ...none, kind: 'messages', input: [1], output: [1], messages: [1], tests: [1] })).toEqual(['messages', 'checks'])
  })
  it('offers only Output for any other step, and leaves out tabs with nothing in them', () => {
    expect(dockTabs({ ...none, kind: 'step', output: [1] })).toEqual(['output'])
    expect(dockTabs({ ...none, kind: 'request', output: [1] })).toEqual(['output'])
  })
  it('adds Problems when the step reported some', () => {
    expect(dockTabs({ ...none, kind: 'step', output: [1], diags: ['x'] })).toEqual(['output', 'problems'])
  })
})
