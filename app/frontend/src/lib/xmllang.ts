// XML for the code editor and the read-only views: a small stream tokenizer, so the window needs no language package
// for it. Tag names, attribute names and values, comments, the declaration, CDATA and entities get their own colours;
// it does not check anything (xml.ts does).
import { StreamLanguage, type StringStream } from '@codemirror/language'
import { tags } from '@lezer/highlight'

interface State {
  mode: 'text' | 'comment' | 'cdata' | 'meta' | 'tag'
  quote: string
  named: boolean // the tag's name has been read, so the next name is an attribute
}

function token(stream: StringStream, state: State): string | null {
  switch (state.mode) {
    case 'comment':
      return closeAt(stream, state, '-->', 'comment')
    case 'cdata':
      return closeAt(stream, state, ']]>', 'string')
    case 'meta':
      return closeAt(stream, state, state.quote === '?' ? '?>' : '>', 'meta')
    case 'tag':
      return tagToken(stream, state)
  }
  return textToken(stream, state)
}

// Reads up to and including end, or to the end of the line when it is not on this one.
function closeAt(stream: StringStream, state: State, end: string, name: string): string {
  const rest = stream.string.slice(stream.pos)
  const at = rest.indexOf(end)
  if (at < 0) stream.skipToEnd()
  else {
    stream.pos += at + end.length
    state.mode = 'text'
    state.quote = '' // meta reuses it to remember which kind of section it is in
  }
  return name
}

function textToken(stream: StringStream, state: State): string | null {
  if (stream.match('<!--')) {
    state.mode = 'comment'
    return 'comment'
  }
  if (stream.match('<![CDATA[')) {
    state.mode = 'cdata'
    return 'string'
  }
  if (stream.match('<?')) {
    state.mode = 'meta'
    state.quote = '?'
    return 'meta'
  }
  if (stream.match('<!')) {
    state.mode = 'meta'
    state.quote = '!'
    return 'meta'
  }
  if (stream.match(/^<\/?/)) {
    state.mode = 'tag'
    state.named = false
    return 'xmlPunct'
  }
  if (stream.match(/^&[^;\s<]*;?/)) return 'xmlEntity'
  stream.match(/^[^<&]+/) || stream.next()
  return null
}

function tagToken(stream: StringStream, state: State): string | null {
  if (state.quote) {
    const rest = stream.string.slice(stream.pos)
    const at = rest.indexOf(state.quote)
    if (at < 0) stream.skipToEnd()
    else {
      stream.pos += at + 1
      state.quote = ''
    }
    return 'xmlString'
  }
  if (stream.eatSpace()) return null
  if (stream.match('/>') || stream.eat('>')) {
    state.mode = 'text'
    return 'xmlPunct'
  }
  if (stream.eat('=')) return 'xmlPunct'
  const q = stream.peek()
  if (q === '"' || q === "'") {
    stream.next()
    state.quote = q
    return tagToken(stream, state) && 'xmlString'
  }
  if (stream.match(/^[^\s=/>"']+/)) {
    const name = state.named ? 'xmlAttr' : 'xmlTag'
    state.named = true
    return name
  }
  stream.next()
  return null
}

export const xmlLanguage = StreamLanguage.define<State>({
  name: 'xml',
  startState: () => ({ mode: 'text', quote: '', named: false }),
  copyState: (s) => ({ ...s }),
  token,
  // The names map onto the tags the theme colours (app.css): tag names as keywords, attributes as keys.
  tokenTable: {
    xmlTag: tags.keyword,
    xmlAttr: tags.propertyName,
    xmlString: tags.string,
    xmlPunct: tags.punctuation,
    xmlEntity: tags.number,
    comment: tags.comment,
    string: tags.string,
    meta: tags.meta,
  },
})
