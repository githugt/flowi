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

describe('highlight of a curl command', () => {
  const curl = `curl -X POST https://api.example.com/pets \\
  -H 'Content-Type: application/json' \\
  -d '{"name": "Rex", "age": 3}'`

  it.each([
    { name: 'the command', part: 'curl', want: 'tok-keyword' },
    { name: 'a flag', part: '-X', want: 'tok-number' },
    { name: 'the method', part: 'POST', want: 'tok-keyword' },
    { name: 'the URL', part: 'https://api.example.com/pets', want: 'tok-string' },
    { name: 'a header name', part: 'Content-Type', want: 'tok-propertyName' },
    { name: 'a header value', part: 'application/json', want: 'tok-string' },
    { name: 'a JSON key in the body', part: '"name"', want: 'tok-propertyName' },
    { name: 'a JSON number in the body', part: '3', want: 'tok-number' },
  ])('should colour $name', ({ part, want }) => {
    const got = classOf(curl, 'curl', part)

    expect(got).toContain(want)
  })

  it('should keep every character, in order', () => {
    const got = highlight(curl, 'curl').map((s) => s.text).join('')

    expect(got).toBe(curl)
  })
})

describe('highlight of a .http request', () => {
  const http = 'POST https://api.example.com/pets HTTP/1.1\nContent-Type: application/json\n\n{"name": "Rex"}\n### next\nGET {{base}}/pets'

  it.each([
    { name: 'the method', part: 'POST', want: 'tok-keyword' },
    { name: 'the URL', part: 'https://api.example.com/pets', want: 'tok-string' },
    { name: 'a header name', part: 'Content-Type', want: 'tok-propertyName' },
    { name: 'a JSON key in the body', part: '"name"', want: 'tok-propertyName' },
    { name: 'a separator', part: '### next', want: 'tok-comment' },
    { name: 'a template in a URL', part: '{{base}}', want: 'tok-template' },
  ])('should colour $name', ({ part, want }) => {
    const got = classOf(http, 'http', part)

    expect(got).toContain(want)
  })

  it('should keep every character, in order', () => {
    const got = highlight(http, 'http').map((s) => s.text).join('')

    expect(got).toBe(http)
  })
})

describe('highlight of XML', () => {
  const xml = '<?xml version="1.0"?>\n<!-- note -->\n<D:prop xmlns:D="DAV:" id="{{id}}"><D:name>a &amp; b</D:name><![CDATA[x < y]]></D:prop>'

  it.each([
    { name: 'a tag name', part: 'D:prop', want: 'tok-keyword' },
    { name: 'an attribute name', part: 'xmlns:D', want: 'tok-propertyName' },
    { name: 'an attribute value', part: '"DAV:"', want: 'tok-string' },
    { name: 'the declaration', part: '<?xml version="1.0"?>', want: 'tok-meta' },
    { name: 'a comment', part: '<!-- note -->', want: 'tok-comment' },
    { name: 'an entity', part: '&amp;', want: 'tok-number' },
    { name: 'a CDATA section', part: 'x < y', want: 'tok-string' },
    { name: 'a template in an attribute', part: '{{id}}', want: 'tok-template' },
  ])('should colour $name', ({ part, want }) => {
    const got = classOf(xml, 'xml', part)

    expect(got).toContain(want)
  })

  it('should keep every character, in order', () => {
    const got = highlight(xml, 'xml').map((s) => s.text).join('')

    expect(got).toBe(xml)
  })

  it('should take text starting with < as XML when guessing', () => {
    expect(guessLang('  <a/>')).toBe('xml')
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

describe('highlight of SQL', () => {
  const sql = `-- who ordered
SELECT c.id, "Full Name", count(*) AS n
FROM customers c WHERE c.id = $1 AND region = ? AND note = 'it''s' AND qty > 10 /* more */
LIMIT 5`

  it.each([
    { name: 'a keyword', part: 'SELECT', want: 'tok-keyword' },
    { name: 'a keyword in lower case', text: 'select 1 from t', part: 'select', want: 'tok-keyword' },
    { name: 'a quoted name', part: '"Full Name"', want: 'tok-propertyName' },
    { name: 'a numbered placeholder', part: '$1', want: 'tok-number' },
    { name: 'a question mark placeholder', part: '?', want: 'tok-number' },
    { name: 'a string with a doubled quote', part: "'it''s'", want: 'tok-string' },
    { name: 'a number', part: '10', want: 'tok-number' },
    { name: 'a line comment', part: '-- who ordered', want: 'tok-comment' },
    { name: 'a block comment', part: '/* more */', want: 'tok-comment' },
  ])('should colour $name', ({ text, part, want }) => {
    const got = classOf(text ?? sql, 'sql', part)

    expect(got).toContain(want)
  })

  it('should leave a table name uncoloured', () => {
    const got = classOf(sql, 'sql', 'customers')

    expect(got).toBe('')
  })

  it('should not take a word inside a string for a keyword', () => {
    const got = classOf("SELECT 'select from where'", 'sql', "'select from where'")

    expect(got).toContain('tok-string')
  })

  it('should colour a template inside a string', () => {
    const got = classOf("SELECT * FROM t WHERE a = '{{x}}'", 'sql', '{{x}}')

    expect(got).toContain('tok-template')
  })

  it('should keep every character, in order', () => {
    const got = highlight(sql, 'sql').map((s) => s.text).join('')

    expect(got).toBe(sql)
  })
})
