// Syntax colouring shared by the code editor and the read-only code views. Both mark text with
// @lezer/highlight's classHighlighter classes (tok-string, tok-number, …), which app.css colours from
// theme tokens, and mark {{templates}} with tok-template.
import { highlightTree, classHighlighter } from '@lezer/highlight'
import { jsonLanguage } from '@codemirror/lang-json'
import { yamlLanguage } from '@codemirror/lang-yaml'

export type CodeLang = 'json' | 'yaml' | 'text'

export interface Segment {
  text: string
  cls: string
}

/** Templates are coloured in any language: they are Flowi's own, inside JSON strings as much as in text. */
export const templatePattern = /\{\{[^{}]*\}\}/g

/** The language of text shown without one: JSON when it starts like JSON, else plain text. */
export function guessLang(text: string): CodeLang {
  const start = text.trimStart()[0]
  return start === '{' || start === '[' ? 'json' : 'text'
}

const parsers = { json: jsonLanguage.parser, yaml: yamlLanguage.parser }

/** Splits text into runs that share one set of classes. */
export function highlight(text: string, lang: CodeLang): Segment[] {
  const classes: string[] = new Array(text.length).fill('')
  if (lang !== 'text') {
    highlightTree(parsers[lang].parse(text), classHighlighter, (from, to, cls) => {
      for (let i = from; i < to; i++) classes[i] = cls
    })
  }
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
