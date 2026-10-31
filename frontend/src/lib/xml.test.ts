import { describe, expect, it } from 'vitest'
import { multiStatus, parseXml, pathsOf, type XmlElement } from './xml'

const ok = (text: string): XmlElement => {
  const got = parseXml(text)
  if ('error' in got) throw new Error(`${got.error.line}:${got.error.column} ${got.error.message}`)
  return got.root
}

describe('parseXml', () => {
  it('should read elements, attributes and text', () => {
    const root = ok('<?xml version="1.0"?>\n<!-- c -->\n<a x="1" y=\'two\'><b>hi &amp; bye</b><c/></a>')

    expect(root.name).toBe('a')
    expect(root.attrs).toEqual([{ name: 'x', value: '1' }, { name: 'y', value: 'two' }])
    expect(root.children.map((c) => c.name)).toEqual(['b', 'c'])
    expect(root.children[0].text).toBe('hi & bye')
  })

  it('should keep CDATA as text and skip a doctype', () => {
    const root = ok('<!DOCTYPE a [<!ENTITY x "y">]><a><![CDATA[1 < 2]]></a>')

    expect(root.text).toBe('1 < 2')
  })

  it.each([
    { name: 'a mismatched end tag', text: '<a><b></a></b>', line: 1, message: /<\/b>|<\/a>/ },
    { name: 'an unclosed element', text: '<a>\n<b>\n', line: 2, message: /<b> is not closed/ },
    { name: 'two roots', text: '<a/><b/>', line: 1, message: /one root/ },
    { name: 'text before the root', text: 'hello <a/>', line: 1, message: /outside/ },
    { name: 'an unquoted attribute', text: '<a x=1/>', line: 1, message: /quote/ },
    { name: 'nothing', text: '  ', line: 1, message: /no element/ },
  ])('should report $name with its line', ({ text, line, message }) => {
    const got = parseXml(text)

    expect('error' in got).toBe(true)
    if ('error' in got) {
      expect(got.error.line).toBe(line)
      expect(got.error.message).toMatch(message)
    }
  })

  it('should leave a {{template}} in text and attribute values alone', () => {
    const root = ok('<a id="{{id}}">{{name}}</a>')

    expect(root.attrs[0].value).toBe('{{id}}')
    expect(root.text).toBe('{{name}}')
  })
})

describe('pathsOf', () => {
  it('should number only siblings that share a name', () => {
    const root = ok('<D:multistatus><D:response/><D:response/><D:other/></D:multistatus>')

    const got = pathsOf(root)

    expect(got.get(root)).toBe('/D:multistatus')
    expect(got.get(root.children[1])).toBe('/D:multistatus/D:response[2]')
    expect(got.get(root.children[2])).toBe('/D:multistatus/D:other')
  })
})

const ms = `<?xml version="1.0"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/files/</D:href>
    <D:propstat>
      <D:prop><D:displayname>files</D:displayname><D:resourcetype><D:collection/></D:resourcetype></D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
    <D:propstat>
      <D:prop><D:getetag/></D:prop>
      <D:status>HTTP/1.1 404 Not Found</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/files/gone.txt</D:href>
    <D:status>HTTP/1.1 410 Gone</D:status>
  </D:response>
</D:multistatus>`

describe('multiStatus', () => {
  it('should list each response with its href, status and properties', () => {
    const got = multiStatus(ok(ms))

    expect(got).toHaveLength(2)
    expect(got![0].href).toBe('/files/')
    expect(got![0].props).toEqual([
      { name: 'D:displayname', value: 'files', status: 'HTTP/1.1 200 OK' },
      { name: 'D:resourcetype', value: '<D:collection/>', status: 'HTTP/1.1 200 OK' },
      { name: 'D:getetag', value: '', status: 'HTTP/1.1 404 Not Found' },
    ])
    expect(got![1]).toEqual({ href: '/files/gone.txt', status: 'HTTP/1.1 410 Gone', props: [] })
  })

  it('should be null for a document that is not a multistatus', () => {
    expect(multiStatus(ok('<a/>'))).toBeNull()
  })
})
