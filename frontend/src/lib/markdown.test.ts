import { describe, expect, it } from 'vitest'
import { renderMarkdown } from './markdown'

describe('renderMarkdown', () => {
  it('escapes markup in the source', () => {
    expect(renderMarkdown('<script>alert(1)</script>')).not.toContain('<script>')
    expect(renderMarkdown('a & b')).toContain('a &amp; b')
  })

  it('renders headings, lists, bold and code', () => {
    const html = renderMarkdown('# Title\n\n- one\n- **two** and `code`\n')
    expect(html).toContain('<h2>Title</h2>')
    expect(html).toContain('<ul>')
    expect(html).toContain('<strong>two</strong>')
    expect(html).toContain('<code>code</code>')
  })

  it('keeps only https links and never as a real href', () => {
    const html = renderMarkdown('[ok](https://example.test/x) [bad](javascript:alert(1))')
    expect(html).toContain('data-href="https://example.test/x"')
    expect(html).not.toContain('javascript:alert(1)"')
    expect(html).toContain('href="#"')
  })
})
