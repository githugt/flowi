import { describe, expect, it } from 'vitest'
import pane from '../ResponsePane.svelte?raw'
import media from '../BodyMedia.svelte?raw'

describe('response body viewers wiring', () => {
  it('should preview a page in an iframe with an empty sandbox, so a response page cannot run script', () => {
    // Before: HTML, images, PDFs and binary bodies were all drawn as plain text.
    expect(pane).toMatch(/<iframe[^>]*sandbox=""[^>]*srcdoc=\{r\.bodyHtml\}/)
    expect(pane).not.toMatch(/sandbox="[^"]+"/)
  })

  it('should draw the media viewer from the body kind and offer Save body for what is not shown', () => {
    expect(pane).toContain('<BodyMedia {r} onsave={saveBody} />')
    expect(media).toContain('src={r.bodyImage}')
    expect(media).toContain('Save body…')
  })
})
