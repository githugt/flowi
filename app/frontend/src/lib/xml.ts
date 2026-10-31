// A small XML reader for what the window shows: a well-formedness check with the line of the first problem, a tree to
// browse, paths to copy and the multi-status list of a WebDAV answer. It does not resolve namespaces (a prefix stays
// as written), read DTDs or check entities; {{templates}} pass through as text. The window has no DOM parser in its
// tests, and an error with a line is more use than the browser's parsererror document.

export interface XmlElement {
  name: string
  attrs: { name: string; value: string }[]
  children: XmlElement[]
  /** The element's own text with entities decoded, trimmed; its children's text is in them. */
  text: string
  line: number
}

export interface XmlError {
  message: string
  line: number
  column: number
}

export type XmlResult = { root: XmlElement } | { error: XmlError }

const entities: Record<string, string> = { amp: '&', lt: '<', gt: '>', quot: '"', apos: "'" }

/** Reads text as one XML document: the first problem is reported with its line and column. */
export function parseXml(text: string): XmlResult {
  const reader = new Reader(text)
  try {
    return { root: reader.document() }
  } catch (e) {
    if (e instanceof ReadError) return { error: e.error }
    throw e
  }
}

class ReadError extends Error {
  constructor(readonly error: XmlError) {
    super(error.message)
  }
}

class Reader {
  private i = 0
  constructor(private readonly s: string) {}

  document(): XmlElement {
    let root: XmlElement | null = null
    for (;;) {
      this.skipIgnorable()
      if (this.i >= this.s.length) break
      if (this.s[this.i] !== '<') throw this.fail('text outside the root element')
      if (root) throw this.fail('a document has one root element')
      root = this.element()
    }
    if (!root) throw this.fail('no element in the document')
    return root
  }

  // Whitespace, comments, the XML declaration and processing instructions, and a doctype: nothing the tree holds.
  private skipIgnorable() {
    for (;;) {
      this.skipSpace()
      if (this.s.startsWith('<!--', this.i)) this.skipTo('-->', 'a comment is not closed')
      else if (this.s.startsWith('<?', this.i)) this.skipTo('?>', 'a processing instruction is not closed')
      else if (this.s.startsWith('<!DOCTYPE', this.i)) this.skipDoctype()
      else return
    }
  }

  private skipDoctype() {
    let depth = 0
    for (; this.i < this.s.length; this.i++) {
      if (this.s[this.i] === '[') depth++
      else if (this.s[this.i] === ']') depth--
      else if (this.s[this.i] === '>' && depth <= 0) {
        this.i++
        return
      }
    }
    throw this.fail('the doctype is not closed')
  }

  private skipTo(end: string, message: string) {
    const at = this.s.indexOf(end, this.i)
    if (at < 0) throw this.fail(message)
    this.i = at + end.length
  }

  private skipSpace() {
    while (this.i < this.s.length && /\s/.test(this.s[this.i])) this.i++
  }

  private element(): XmlElement {
    const line = this.lineAt(this.i)
    this.i++ // <
    const name = this.name()
    const el: XmlElement = { name, attrs: [], children: [], text: '', line }
    for (;;) {
      this.skipSpace()
      if (this.s.startsWith('/>', this.i)) {
        this.i += 2
        return el
      }
      if (this.s[this.i] === '>') {
        this.i++
        break
      }
      if (this.i >= this.s.length) throw this.fail(`<${name}> is not closed`, line)
      el.attrs.push(this.attribute())
    }
    this.content(el)
    return el
  }

  private attribute(): { name: string; value: string } {
    const name = this.name()
    this.skipSpace()
    if (this.s[this.i] !== '=') throw this.fail(`attribute ${name} has no value`)
    this.i++
    this.skipSpace()
    const quote = this.s[this.i]
    if (quote !== '"' && quote !== "'") throw this.fail(`the value of ${name} must be in quotes`)
    const end = this.s.indexOf(quote, this.i + 1)
    if (end < 0) throw this.fail(`the value of ${name} is not closed`)
    const value = decode(this.s.slice(this.i + 1, end))
    this.i = end + 1
    return { name, value }
  }

  private content(el: XmlElement) {
    let text = ''
    for (;;) {
      const next = this.s.indexOf('<', this.i)
      if (next < 0) throw this.fail(`<${el.name}> is not closed`, el.line, this.s.length)
      text += decode(this.s.slice(this.i, next))
      this.i = next
      if (this.s.startsWith('</', this.i)) {
        this.i += 2
        const close = this.name()
        this.skipSpace()
        if (this.s[this.i] !== '>') throw this.fail(`the end tag </${close}> is not closed`)
        this.i++
        if (close !== el.name) throw this.fail(`</${close}> closes <${el.name}>`, this.lineAt(next))
        el.text = text.trim()
        return
      }
      if (this.s.startsWith('<!--', this.i)) this.skipTo('-->', 'a comment is not closed')
      else if (this.s.startsWith('<![CDATA[', this.i)) {
        const at = this.s.indexOf(']]>', this.i)
        if (at < 0) throw this.fail('a CDATA section is not closed')
        text += this.s.slice(this.i + 9, at)
        this.i = at + 3
      } else if (this.s.startsWith('<?', this.i)) this.skipTo('?>', 'a processing instruction is not closed')
      else el.children.push(this.element())
    }
  }

  private name(): string {
    const m = /^[^\s=/>"'<]+/.exec(this.s.slice(this.i))
    if (!m) throw this.fail('a name was expected')
    this.i += m[0].length
    return m[0]
  }

  private lineAt(index: number): number {
    return this.s.slice(0, index).split('\n').length
  }

  private fail(message: string, line = this.lineAt(this.i), at = this.i): ReadError {
    const before = this.s.slice(0, at)
    return new ReadError({ message, line, column: at - before.lastIndexOf('\n') })
  }
}

function decode(text: string): string {
  return text.replace(/&(#x[0-9a-f]+|#\d+|\w+);/gi, (whole, name: string) => {
    if (name[0] === '#') {
      const code = name[1].toLowerCase() === 'x' ? parseInt(name.slice(2), 16) : parseInt(name.slice(1), 10)
      return Number.isFinite(code) ? String.fromCodePoint(code) : whole
    }
    return entities[name] ?? whole
  })
}

/** An XPath-like path for every element, to copy: a step is numbered only when a sibling has the same name. */
export function pathsOf(root: XmlElement): Map<XmlElement, string> {
  const out = new Map<XmlElement, string>()
  const walk = (el: XmlElement, path: string) => {
    out.set(el, path)
    for (const child of el.children) {
      const same = el.children.filter((c) => c.name === child.name)
      const step = same.length > 1 ? `${child.name}[${same.indexOf(child) + 1}]` : child.name
      walk(child, `${path}/${step}`)
    }
  }
  walk(root, '/' + root.name)
  return out
}

export interface MultiStatusEntry {
  href: string
  /** The status of a response that has one of its own (a resource that is gone), else "". */
  status: string
  props: { name: string; value: string; status: string }[]
}

const localName = (name: string) => name.slice(name.indexOf(':') + 1)
const child = (el: XmlElement, name: string) => el.children.find((c) => localName(c.name) === name)

/** The responses of a WebDAV 207 Multi-Status (RFC 4918): each with its href and the properties by status. Null when the document is not one. */
export function multiStatus(root: XmlElement): MultiStatusEntry[] | null {
  if (localName(root.name) !== 'multistatus') return null
  return root.children
    .filter((r) => localName(r.name) === 'response')
    .map((r) => {
      const props: MultiStatusEntry['props'] = []
      for (const stat of r.children.filter((c) => localName(c.name) === 'propstat')) {
        const status = child(stat, 'status')?.text ?? ''
        for (const prop of child(stat, 'prop')?.children ?? []) props.push({ name: prop.name, value: propValue(prop), status })
      }
      return { href: child(r, 'href')?.text ?? '', status: child(r, 'status')?.text ?? '', props }
    })
}

// A property's value: its text, or the names of its children when it holds elements (resourcetype is <collection/>).
function propValue(prop: XmlElement): string {
  if (prop.text || !prop.children.length) return prop.text
  return prop.children.map((c) => `<${c.name}/>`).join('')
}
