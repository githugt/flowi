import { describe, expect, it } from 'vitest'
import { lastPassText, sparkPoints } from './watchpasses'
import type { WatchPass } from './types'

const pass = (over: Partial<WatchPass> = {}): WatchPass => ({ at: '2026-11-03T10:00:00Z', status: 'passed', changes: 0, ms: 100, ...over })

describe('lastPassText', () => {
  it('says there is no pass yet when the watch has not run', () => {
    expect(lastPassText([])).toEqual({ text: 'No pass yet', tone: 'none' })
  })

  it('reports the latest pass only', () => {
    const history = [pass({ changes: 3 }), pass()]

    expect(lastPassText(history)).toEqual({ text: 'No change', tone: 'ok' })
  })

  it('counts changes, singular and plural', () => {
    expect(lastPassText([pass({ changes: 1 })]).text).toBe('1 change')
    expect(lastPassText([pass({ changes: 4 })]).text).toBe('4 changes')
  })

  it('calls a run that did not pass what it was, whatever it compared', () => {
    expect(lastPassText([pass({ status: 'failed', changes: 2 })])).toEqual({ text: 'Run failed', tone: 'bad' })
  })
})

describe('sparkPoints', () => {
  it('spreads the passes over the width and puts the slowest at the top', () => {
    const points = sparkPoints([pass({ ms: 50 }), pass({ ms: 100 }), pass({ ms: 0 })], 100, 20)

    expect(points.map((p) => [p.x, p.y])).toEqual([[0, 10], [50, 0], [100, 20]])
  })

  it('draws a single pass as a dot in the middle', () => {
    expect(sparkPoints([pass()], 100, 20)).toEqual([{ x: 50, y: 10, changed: false }])
  })

  it('marks the passes that found a change', () => {
    const points = sparkPoints([pass(), pass({ changes: 1 }), pass({ status: 'failed', changes: 1 })], 100, 20)

    expect(points.map((p) => p.changed)).toEqual([false, true, false])
  })

  it('has nothing to draw without passes', () => {
    expect(sparkPoints([], 100, 20)).toEqual([])
  })
})
