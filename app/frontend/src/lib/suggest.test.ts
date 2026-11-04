import { describe, expect, it } from 'vitest'
import { miniLayout, shapeText, sureText, surestFirst, takeText, weakestLink } from './suggest'
import type { FlowSuggestion } from './types'

const suggestion = (name: string, ...confidences: number[]): FlowSuggestion => ({
  name,
  path: `suggested/${name}`,
  steps: [
    { id: 'create', request: 'create', method: 'POST', takes: [] },
    { id: 'get', request: 'get', method: 'GET', takes: confidences.map((confidence, i) => ({ param: `p${i}`, from: 'create', field: 'id', confidence })) },
  ],
})

describe('suggest', () => {
  it('says which field feeds which parameter', () => {
    expect(takeText({ param: 'petId', from: 'create', field: 'id', confidence: 0.9 })).toBe('petId from create.id')
  })

  it('says the confidence as a percentage', () => {
    expect(sureText(0.9)).toBe('90% sure')
    expect(sureText(0.847)).toBe('85% sure')
  })

  it('judges a suggestion by its least sure link, and a flow with no links by 1', () => {
    expect(weakestLink(suggestion('a', 0.95, 0.8))).toBe(0.8)
    expect(weakestLink(suggestion('none'))).toBe(1)
  })

  it('lists the surest suggestion first, ties by name', () => {
    const got = surestFirst([suggestion('b', 0.8), suggestion('c', 0.95), suggestion('a', 0.8)])

    expect(got.map((s) => s.name)).toEqual(['c', 'a', 'b'])
  })

  it('does not reorder the list it is given', () => {
    const list = [suggestion('b', 0.8), suggestion('a', 0.95)]

    surestFirst(list)

    expect(list.map((s) => s.name)).toEqual(['b', 'a'])
  })

  it('shows the methods in order', () => {
    expect(shapeText(suggestion('a', 0.9))).toBe('POST → GET')
  })
})

describe('miniLayout', () => {
  const chain = (): FlowSuggestion => ({
    name: 'pets',
    path: 'suggested/pets',
    steps: [
      { id: 'create', request: 'pets/create', method: 'POST', takes: [] },
      { id: 'get', request: 'pets/get', method: 'GET', takes: [{ param: 'petId', from: 'create', field: 'id', confidence: 0.9 }] },
      { id: 'delete', request: 'pets/delete', method: 'DELETE', takes: [{ param: 'petId', from: 'create', field: 'id', confidence: 0.6 }] },
    ],
  })

  it('puts the steps in a row, left to right, labelled by their verb', () => {
    const m = miniLayout(chain())

    expect(m.nodes.map((n) => n.label)).toEqual(['POST', 'GET', 'DELETE'])
    expect(m.nodes[1].x).toBeGreaterThan(m.nodes[0].x)
    expect(m.nodes[2].x).toBeGreaterThan(m.nodes[1].x)
    expect(m.width).toBeGreaterThanOrEqual(m.nodes[2].x + m.nodes[2].w)
  })

  it('draws a link from the step a value comes from to the step that takes it', () => {
    const m = miniLayout(chain())

    expect(m.links.map((l) => [l.from, l.to, l.label])).toEqual([
      ['create', 'get', 'petId'],
      ['create', 'delete', 'petId'],
    ])
  })

  it('curves a link over the steps between, and leaves a link between neighbours straight', () => {
    const [next, far] = miniLayout(chain()).links

    expect(next.path).not.toContain('Q')
    expect(far.path).toContain('Q')
  })

  it('marks a link that is not very sure', () => {
    const [sure, unsure] = miniLayout(chain()).links

    expect(sure.weak).toBe(false)
    expect(unsure.weak).toBe(true)
  })

  it('ignores a link from a step the flow does not have', () => {
    const s = chain()
    s.steps[1].takes[0].from = 'ghost'

    expect(miniLayout(s).links.map((l) => l.to)).toEqual(['delete'])
  })
})
