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

  it('renders numbered lists and quotes, and keeps their text escaped', () => {
    const html = renderMarkdown('1. first <b>\n2) second\n\n> quoted & **bold**')
    expect(html).toContain('<ol>')
    expect(html).toContain('<li>first &lt;b&gt;</li>')
    expect(html).toContain('<li>second</li>')
    expect(html).toContain('<blockquote>quoted &amp; <strong>bold</strong></blockquote>')
    expect(html.match(/<ol>/g)).toHaveLength(1)
  })

  it('closes a list before another kind of list starts', () => {
    const html = renderMarkdown('- a\n1. b')
    expect(html).toBe('<ul>\n<li>a</li>\n</ul>\n<ol>\n<li>b</li>\n</ol>')
  })
})
