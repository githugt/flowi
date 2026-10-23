import { describe, expect, it } from 'vitest'
import { guessLang, highlight } from './highlight'

const classOf = (text: string, lang: Parameters<typeof highlight>[1], part: string) =>
  highlight(text, lang).find((s) => s.text.includes(part))?.cls ?? ''

describe('highlight', () => {
  it.each([
    { name: 'a JSON key', text: '{"name": "Rex", "age": 3}', lang: 'json' as const, part: '"name"', want: 'tok-propertyName' },
    { name: 'a JSON string', text: '{"name": "Rex"}', lang: 'json' as const, part: '"Rex"', want: 'tok-string' },
    { name: 'a JSON number', text: '{"age": 3}', lang: 'json' as const, part: '3', want: 'tok-number' },
    { name: 'a template inside a JSON string', text: '{"id": "{{petId}}"}', lang: 'json' as const, part: '{{petId}}', want: 'tok-template' },
    { name: 'a template in plain text', text: 'Bearer {{secret.token}}', lang: 'text' as const, part: '{{secret.token}}', want: 'tok-template' },
  ])('should colour $name', ({ text, lang, part, want }) => {
    const got = classOf(text, lang, part)

    expect(got).toContain(want)
  })

  it('should keep every character, in order', () => {
    const text = '{"a": [1, true, null], "b": "{{x}}"}'

    const got = highlight(text, 'json').map((s) => s.text).join('')

    expect(got).toBe(text)
  })
})

describe('guessLang', () => {
  it.each([
    ['  {"a": 1}', 'json'],
    ['[1, 2]', 'json'],
    ['plain text', 'text'],
    ['', 'text'],
  ])('should take %j as %s', (text, want) => {
    expect(guessLang(text)).toBe(want)
  })
})
