import { describe, expect, it } from 'vitest'

describe('the retries fields of the step inspector', () => {
  it('should show retries only for a step that can be retried, and the wait only once there are retries', async () => {
    // Before: a step's retries could not be set from the window.
    const src = (await import('./Inspector.svelte?raw')).default as string

    expect(src).toContain('{#if step.canRetry}')
    expect(src).toContain("ontext={(v) => text('retries', v)}")
    expect(src).toMatch(/\{#if step\.retries\}<Field[\s\S]*?text\('retryWait', v\)/)
  })
})
