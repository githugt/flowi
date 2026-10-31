// SQL for the code editor and the read-only views: a small stream tokenizer, so the window needs no language package
// for it. Keywords, strings, quoted names, numbers, placeholders ($1, ?) and comments get their own colours; it does
// not check anything (the store does).
import { StreamLanguage, type StringStream } from '@codemirror/language'
import { tags } from '@lezer/highlight'

interface State {
  inBlockComment: boolean
}

// The words that start or shape a query. Not every word of every dialect: the ones that read as keywords in a SELECT.
const keywords = new Set(
  (
    'select from where and or not in is null like ilike between exists case when then else end as on join inner left right full outer cross ' +
    'group by order having limit offset union all distinct with recursive values table show explain describe desc asc over partition ' +
    'true false using natural fetch first next rows only for window except intersect any some cast'
  ).split(' '),
)

function token(stream: StringStream, state: State): string | null {
  if (state.inBlockComment) return blockComment(stream, state)
  if (stream.eatSpace()) return null
  if (stream.match('--')) {
    stream.skipToEnd()
    return 'comment'
  }
  if (stream.match('/*')) {
    state.inBlockComment = true
    return blockComment(stream, state)
  }
  const c = stream.peek()
  if (c === "'") return quoted(stream, "'", 'string')
  if (c === '"' || c === '`') return quoted(stream, c, 'name')
  if (stream.match(/^\$\d+/) || stream.eat('?')) return 'placeholder'
  if (stream.match(/^\d+(\.\d+)?([eE][+-]?\d+)?/)) return 'number'
  if (stream.match(/^[A-Za-z_][\w$]*/)) return keywords.has(stream.current().toLowerCase()) ? 'keyword' : null
  stream.next()
  return null
}

// Reads to the end of the comment, or of the line when it goes on.
function blockComment(stream: StringStream, state: State): string {
  const rest = stream.string.slice(stream.pos)
  const at = rest.indexOf('*/')
  if (at < 0) stream.skipToEnd()
  else {
    stream.pos += at + 2
    state.inBlockComment = false
  }
  return 'comment'
}

// Reads a string or quoted name; a doubled quote inside stays in it. One that is not closed runs to the end of the line.
function quoted(stream: StringStream, quote: string, name: string): string {
  stream.next()
  while (!stream.eol()) {
    if (stream.next() !== quote) continue
    if (stream.peek() !== quote) break
    stream.next()
  }
  return name
}

export const sqlLanguage = StreamLanguage.define<State>({
  name: 'sql',
  startState: () => ({ inBlockComment: false }),
  copyState: (s) => ({ ...s }),
  token,
  // The names map onto the tags the theme colours (app.css): quoted names as keys, placeholders as numbers.
  tokenTable: {
    keyword: tags.keyword,
    string: tags.string,
    name: tags.propertyName,
    placeholder: tags.number,
    number: tags.number,
    comment: tags.comment,
  },
})
