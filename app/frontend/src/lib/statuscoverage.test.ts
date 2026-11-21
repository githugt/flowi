import { describe, expect, it } from 'vitest'
import { statusMark } from './statusmark'
import stepCard from '../StepCard.svelte?raw'
import canvas from '../Canvas.svelte?raw'

// The test runner has no Node typings; the Go source is read from disk, so a status Go starts to emit has to be known
// here before the tests pass.
// @ts-ignore
const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
const go = readFileSync(new URL('../../../internal/vm/flowrun.go', import.meta.url), 'utf8')

/** The statuses a step or a run can have, in the order Go ranks them (the empty one is "nothing yet"). */
const emitted = (() => {
  const m = /func statusRank[\s\S]*?\[\]string\{([^}]*)\}/.exec(go)
  return [...(m?.[1] ?? '').matchAll(/"([a-z]+)"/g)].map((x) => x[1])
})()

describe('the statuses Go emits for a step', () => {
  // Before: a status the window did not know got a "?" and no colour, found only by running into it.
  it('are read from flowrun.go', () => {
    expect(emitted).toEqual(['skipped', 'passed', 'running', 'cancelled', 'failed', 'error', 'aborted'])
  })

  it('each have a shape, so none is shown by colour alone', () => {
    for (const s of emitted) expect(statusMark(s), s).not.toBe('?')
  })

  it('each are named in the step card\'s styles, and the failing ones also in the canvas', () => {
    for (const s of emitted) expect(stepCard, s).toMatch(new RegExp(`\\.${s}\\b`))
    for (const s of ['passed', 'failed', 'error', 'aborted', 'running']) expect(canvas, s).toMatch(new RegExp(`\\.${s}\\b`))
  })

  it('give a different shape to the ones that differ in meaning', () => {
    const shapes = ['passed', 'failed', 'error', 'running', 'skipped'].map(statusMark)

    expect(new Set(shapes).size).toBe(shapes.length)
  })
})
