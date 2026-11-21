import { describe, expect, it } from 'vitest'
import { runChip } from './runstate'

const flow = (running: boolean) => ({ run: { running } })
const req = (sending: boolean) => ({ sending })

describe('runChip', () => {
  it('Given nothing in progress Then there is no chip', () => {
    // Before: the footer did not say a flow or a send was still going behind another tab.
    expect(runChip([flow(false)], [req(false)])).toBeNull()
    expect(runChip([], [])).toBeNull()
  })
  it('Given a flow running Then it says so', () => {
    expect(runChip([flow(true), flow(false)], [])?.text).toBe('Running 1 flow')
  })
  it('Given several flows and sends Then both are counted', () => {
    const c = runChip([flow(true), flow(true)], [req(true)])

    expect(c?.text).toBe('Running 2 flows · 1 send')
    expect(c?.title).toBe('In progress in the open tabs: 2 flows and 1 send.')
  })
})
