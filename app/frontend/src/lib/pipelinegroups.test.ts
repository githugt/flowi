import { describe, expect, it } from 'vitest'
import { flowEnd, groupPipeline } from './pipelinegroups'

const step = (kind: string, title: string, state = 'passed') => ({ kind, title, state })

describe('groupPipeline', () => {
  it('puts the steps under each heading in a card of their own', () => {
    const rows = [step('section', 'Post-hooks', ''), step('node', 'log'), step('node', 'assert'), step('section', 'After the post-hooks', ''), step('builtin', 'schema')]

    const groups = groupPipeline(rows)

    expect(groups.map((g) => [g.title, g.steps.map((s) => s.title)])).toEqual([
      ['Post-hooks', ['log', 'assert']],
      ['After the post-hooks', ['schema']],
    ])
  })

  it('keeps the request out of the hooks around it', () => {
    const rows = [step('section', 'Pre-hooks', ''), step('node', 'login'), step('api', 'GET /x'), step('section', 'Post-hooks', ''), step('node', 'check')]

    const groups = groupPipeline(rows)

    expect(groups.map((g) => g.title)).toEqual(['Pre-hooks', '', 'Post-hooks'])
  })

  it('drops a heading with no steps', () => {
    expect(groupPipeline([step('section', 'Post-hooks', '')])).toEqual([])
  })

  it('tells how each card went', () => {
    const rows = [step('section', 'A', ''), step('node', 'x'), step('section', 'B', ''), step('node', 'y', 'failed'), step('section', 'C', ''), step('builtin', 'z', 'not-run'), step('section', 'D', ''), step('node', 'w', 'idle')]

    expect(groupPipeline(rows).map((g) => g.tone)).toEqual(['passed', 'failed', 'off', ''])
  })
})

describe('flowEnd', () => {
  // Before: the line ran past a failure to the last step, so a failed run looked as if everything after it had run too.
  it('stops at the step that failed', () => {
    expect(flowEnd(['passed', 'passed', 'failed', 'idle', 'idle'])).toEqual({ end: 2, ran: true })
  })

  it('goes to the last step when all passed', () => {
    expect(flowEnd(['passed', 'passed', 'passed'])).toEqual({ end: 2, ran: true })
  })

  it('leaves out a step that is paused after the ones that ran', () => {
    expect(flowEnd(['passed', 'passed', 'not-run'])).toEqual({ end: 1, ran: true })
  })

  it('shows the whole way before anything has run', () => {
    expect(flowEnd(['idle', 'idle', 'idle'])).toEqual({ end: 2, ran: false })
  })
})
