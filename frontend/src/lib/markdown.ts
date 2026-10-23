// A small, safe Markdown renderer for text we ship or fetch from a signed
// feed: headings, lists, bold, italics, inline code, code blocks and links.
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
  let list = false
  let code = false
  const closeList = () => {
    if (list) out.push('</ul>')
    list = false
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
    if (heading) {
      closeList()
      out.push(`<h${heading[1].length + 1}>${inline(heading[2])}</h${heading[1].length + 1}>`)
    } else if (item) {
      if (!list) out.push('<ul>')
      list = true
      out.push(`<li>${inline(item[1])}</li>`)
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
