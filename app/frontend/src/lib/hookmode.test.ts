import { describe, expect, it } from 'vitest'
import { hookBadge } from './hookmode'

describe('hookBadge', () => {
  it('says nothing for the usual mode', () => {
    expect(hookBadge('all')).toBeNull()
    expect(hookBadge('')).toBeNull()
  })
  it('names what is skipped for each other mode', () => {
    // Before: no badge existed; only the Send chevron's menu showed the mode.
    expect(hookBadge('none')?.text).toBe('No hooks')
    expect(hookBadge('skip-pre')?.text).toBe('No pre-hooks')
    expect(hookBadge('skip-post')?.text).toBe('No post-hooks')
    expect(hookBadge('none')?.title).toContain('Click to run them again')
  })
})

// @ts-ignore
import editor from '../RequestEditor.svelte?raw'

describe('request header', () => {
  // Before: Discard appeared only while dirty and pushed the version picker and Save along, and Save was an icon alone.
  it('keeps room for the Edited pill and Discard, and labels Save', () => {
    expect(editor).toMatch(/class="edited" class:clean=\{!view\.dirty\}/)
    expect(editor).toMatch(/\.edited\.clean \{ visibility: hidden/)
    expect(editor).toContain('<span class="savetext">Save</span>')
  })
})
