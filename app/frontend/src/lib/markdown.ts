// A small, safe Markdown renderer for text we ship, fetch from a signed feed or
// read from a request's notes (which an import can fill from a spec): headings,
// bulleted and numbered lists, quotes, bold, italics, inline code, code blocks
// and links.
// Everything is escaped first, so the result holds no markup of the source's.

const escapeHTML = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')

function inline(text: string): string {
  let s = escapeHTML(text)
  s = s.replace(/`([^`]+)`/g, '<code>$1</code>')
  s = s.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  s = s.replace(/(^|[\s(])\*([^*\s][^*]*)\*/g, '$1<em>$2</em>')
  // Links: https only, opened by the shell (data-href), never navigated to by the page.
  s = s.replace(/\[([^\]]+)\]\((https:\/\/[^)\s]+)\)/g, '<a href="#" data-href="$2">$1</a>')
  return s
}

export function renderMarkdown(source: string): string {
  const out: string[] = []
  let list: 'ul' | 'ol' | null = null
  let code = false
  const closeList = () => {
    if (list) out.push(`</${list}>`)
    list = null
  }
  const openList = (kind: 'ul' | 'ol') => {
    if (list === kind) return
    closeList()
    out.push(`<${kind}>`)
    list = kind
  }
  for (const line of source.split('\n')) {
    if (line.startsWith('```')) {
      closeList()
      out.push(code ? '</pre>' : '<pre>')
      code = !code
      continue
    }
    if (code) {
      out.push(escapeHTML(line) + '\n')
      continue
    }
    const heading = /^(#{1,4})\s+(.*)$/.exec(line)
    const item = /^\s*[-*]\s+(.*)$/.exec(line)
    const numbered = /^\s*\d+[.)]\s+(.*)$/.exec(line)
    const quote = /^>\s?(.*)$/.exec(line)
    if (heading) {
      closeList()
      out.push(`<h${heading[1].length + 1}>${inline(heading[2])}</h${heading[1].length + 1}>`)
    } else if (item) {
      openList('ul')
      out.push(`<li>${inline(item[1])}</li>`)
    } else if (numbered) {
      openList('ol')
      out.push(`<li>${inline(numbered[1])}</li>`)
    } else if (quote) {
      closeList()
      out.push(`<blockquote>${inline(quote[1])}</blockquote>`)
    } else if (line.trim() === '') {
      closeList()
    } else {
      closeList()
      out.push(`<p>${inline(line)}</p>`)
    }
  }
  closeList()
  if (code) out.push('</pre>')
  return out.join('\n')
}
