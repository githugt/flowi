import { describe, expect, it } from 'vitest'
import { noServices, serviceChips } from './services'

describe('the footer service chips', () => {
  it('should show nothing while every service is stopped', () => {
    expect(serviceChips(noServices)).toEqual([])
  })

  it('should show a running replayer with its address and a running schedule runner with what is due', () => {
    // Before: leaving the screen of a started replayer or schedule runner left nothing to say it was still working.
    const chips = serviceChips({ replayer: { state: 'running', addr: '127.0.0.1:8080' }, schedules: { state: 'running', triggerAddr: '127.0.0.1:9000', upcoming: 3 } })

    expect(chips.map((c) => [c.id, c.text, c.screen, c.busy])).toEqual([
      ['replayer', 'Replayer 127.0.0.1:8080', 'screen:replayer', false],
      ['schedules', 'Schedules · 3 due', 'screen:schedules', false],
    ])
    expect(chips[0].title).toContain('127.0.0.1:8080')
    expect(chips[1].title).toContain('triggers listen on 127.0.0.1:9000')
  })

  it('should show a service that is starting or stopping as busy', () => {
    const chips = serviceChips({ replayer: { state: 'starting', addr: '' }, schedules: { state: 'stopping', triggerAddr: '', upcoming: 0 } })

    expect(chips.map((c) => [c.text, c.busy])).toEqual([
      ['Replayer starting…', true],
      ['Schedules stopping…', true],
    ])
  })
})
