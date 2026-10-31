// Syntax colouring shared by the code editor and the read-only code views. Both mark text with
// @lezer/highlight's classHighlighter classes (tok-string, tok-number, …), which app.css colours from
// theme tokens, and mark {{templates}} with tok-template.
import { highlightTree, classHighlighter } from '@lezer/highlight'
import { jsonLanguage } from '@codemirror/lang-json'
import { yamlLanguage } from '@codemirror/lang-yaml'
import { xmlLanguage } from './xmllang'
import { sqlLanguage } from './sqllang'

/** curl is a shell command line, http a .http file's request: both are coloured by the small readers below. */
export type CodeLang = 'json' | 'yaml' | 'xml' | 'sql' | 'text' | 'curl' | 'http'

export interface Segment {
  text: string
  cls: string
}

/** Templates are coloured in any language: they are Flowi's own, inside JSON strings as much as in text. */
export const templatePattern = /\{\{[^{}]*\}\}/g

/** The language of text shown without one: JSON or XML when it starts like them, else plain text. */
export function guessLang(text: string): CodeLang {
  const start = text.trimStart()[0]
  if (start === '{' || start === '[') return 'json'
  return start === '<' ? 'xml' : 'text'
}

const parsers = { json: jsonLanguage.parser, yaml: yamlLanguage.parser, xml: xmlLanguage.parser, sql: sqlLanguage.parser }

type Mark = (from: number, to: number, cls: string) => void

/** Colours text[from, to) as JSON, the way the body editor does. */
function markJson(text: string, from: number, to: number, mark: Mark) {
  const part = text.slice(from, to)
  highlightTree(parsers.json.parse(part), classHighlighter, (a, b, cls) => mark(from + a, from + b, cls))
}

const curlWord = /'[^']*'|"(?:\\.|[^"\\])*"|\\\n|\S+/g
const curlHeaderFlags = new Set(['-H', '--header'])
const curlBodyFlags = new Set(['-d', '--data', '--data-raw', '--data-binary', '--data-urlencode', '--json'])
const curlMethodFlags = new Set(['-X', '--request'])

/** A curl command line: the command, flags, method, URL, header names and values, and a JSON body in its own colours. */
function markCurl(text: string, mark: Mark) {
  let flag = ''
  for (const m of text.matchAll(curlWord)) {
    const from = m.index
    const to = from + m[0].length
    const word = m[0]
    const quoted = /^['"]/.test(word) && word.length > 1
    const innerFrom = quoted ? from + 1 : from
    const innerTo = quoted ? to - 1 : to
    if (word === '\\\n') {
      mark(from, to, 'tok-punctuation')
    } else if (/^--?[A-Za-z][\w-]*$/.test(word)) {
      mark(from, to, 'tok-number')
      flag = word
      continue
    } else if (word === 'curl' && !flag && from === text.indexOf('curl')) {
      mark(from, to, 'tok-keyword')
    } else if (curlMethodFlags.has(flag)) {
      mark(from, to, 'tok-keyword')
    } else if (curlHeaderFlags.has(flag)) {
      markHeader(text, innerFrom, innerTo, mark)
    } else if (curlBodyFlags.has(flag) && /^\s*[{[]/.test(text.slice(innerFrom, innerTo))) {
      markJson(text, innerFrom, innerTo, mark)
    } else {
      mark(innerFrom, innerTo, 'tok-string')
    }
    flag = ''
  }
}

/** "Name: value": the name as a key, the value as a string. */
function markHeader(text: string, from: number, to: number, mark: Mark) {
  const colon = text.indexOf(':', from)
  if (colon < 0 || colon >= to) {
    mark(from, to, 'tok-string')
    return
  }
  mark(from, colon, 'tok-propertyName')
  mark(colon, colon + 1, 'tok-punctuation')
  mark(colon + 1, to, 'tok-string')
}

const httpRequestLine = /^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS|QUERY|[A-Z][A-Z-]*)(\s+)(\S+)(\s+HTTP\/[\d.]+)?\s*$/

/** A .http file: request lines, header lines, a body after the blank line, ### and # comments. */
function markHttp(text: string, mark: Mark) {
  let at = 0
  let part: 'start' | 'headers' | 'body' = 'start'
  let bodyFrom = -1
  const endBody = (to: number) => {
    if (bodyFrom >= 0 && /^\s*[{[]/.test(text.slice(bodyFrom, to))) markJson(text, bodyFrom, to, mark)
    bodyFrom = -1
  }
  for (const line of text.split('\n')) {
    const from = at
    const to = at + line.length
    at = to + 1
    if (/^\s*(#|\/\/)/.test(line)) {
      endBody(from)
      mark(from, to, 'tok-comment')
      if (/^#{3,}/.test(line)) part = 'start'
    } else if (part === 'body') {
      if (bodyFrom < 0) bodyFrom = from
    } else if (line.trim() === '') {
      if (part === 'headers') part = 'body'
    } else if (part === 'start') {
      const m = httpRequestLine.exec(line)
      if (m) {
        mark(from, from + m[1].length, 'tok-keyword')
        const url = from + m[1].length + m[2].length
        mark(url, url + m[3].length, 'tok-string')
        if (m[4]) mark(url + m[3].length, to, 'tok-meta')
      } else {
        mark(from, to, 'tok-string')
      }
      part = 'headers'
    } else {
      markHeader(text, from, to, mark)
    }
  }
  endBody(text.length)
}

/** Splits text into runs that share one set of classes. */
export function highlight(text: string, lang: CodeLang): Segment[] {
  const classes: string[] = new Array(text.length).fill('')
  const mark: Mark = (from, to, cls) => {
    for (let i = from; i < to; i++) classes[i] = cls
  }
  if (lang === 'curl') markCurl(text, mark)
  else if (lang === 'http') markHttp(text, mark)
  else if (lang !== 'text') highlightTree(parsers[lang].parse(text), classHighlighter, mark)
  for (const m of text.matchAll(templatePattern)) {
    for (let i = m.index; i < m.index + m[0].length; i++) classes[i] = 'tok-template'
  }
  const out: Segment[] = []
  for (let i = 0; i < text.length; i++) {
    const last = out[out.length - 1]
    if (last && last.cls === classes[i]) last.text += text[i]
    else out.push({ text: text[i], cls: classes[i] })
  }
  return out
}
