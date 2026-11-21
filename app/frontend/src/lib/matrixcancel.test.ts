import { describe, expect, it } from 'vitest'
import flowTools from '../FlowTools.svelte?raw'
import requestMatrix from '../RequestMatrix.svelte?raw'
import running from '../MatrixRunning.svelte?raw'
import result from '../MatrixResult.svelte?raw'
import { eventNames } from './events'

describe('matrix progress and cancel wiring', () => {
  it('should show the running line with a Cancel in both matrix dialogs, calling that service\'s CancelMatrix', () => {
    // Before: a matrix showed only a busy button and could not be stopped.
    expect(flowTools).toContain('FlowService.CancelMatrix(node)')
    expect(requestMatrix).toContain('RequestService.CancelMatrix(node)')
    expect(flowTools).toContain('<MatrixRunning')
    expect(requestMatrix).toContain('<MatrixRunning')
  })

  it('should listen to the matrix event, only for its own node', () => {
    expect(eventNames).toContain('matrix')
    expect(running).toContain("onEvent('matrix'")
    expect(running).toContain('d?.node === forNode')
  })

  it('should draw a cancelled matrix as a list, not a grid with holes, and say it was cancelled', () => {
    expect(result).toContain('gridComplete(matrix)')
    expect(result).toContain('stoppedNote(matrix)')
  })
})
