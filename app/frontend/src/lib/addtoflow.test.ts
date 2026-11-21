import { describe, expect, it } from 'vitest'
import { addToFlowTargets, requestOfRow } from './addtoflow'

const tabs = [
  { node: 'f:C1:F1', label: 'signup', kind: 'flow' },
  { node: 'f:C2:F9', label: 'other', kind: 'flow' },
  { node: 'r:C1:R2', label: 'login', kind: 'request' },
]

describe('addToFlowTargets', () => {
  it('lists the open flows of the request’s collection only', () => {
    expect(addToFlowTargets('r:C1:R1', tabs)).toEqual([{ node: 'f:C1:F1', label: 'signup' }])
  })
  it('gives nothing for a row that is not a request, or when no flow is open', () => {
    expect(addToFlowTargets('f:C1:F1', tabs)).toEqual([])
    expect(addToFlowTargets('r:C3:R1', tabs)).toEqual([])
  })
  it('reads a request row', () => {
    expect(requestOfRow('r:C1:R1')).toEqual({ collection: 'C1', id: 'R1' })
    expect(requestOfRow('c:C1')).toBeNull()
  })
})
