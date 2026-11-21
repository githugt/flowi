import { describe, expect, it } from 'vitest'
import { isEditable, recognise, windowPaste } from './paste'
import app from '../App.svelte?raw'

describe('recognise', () => {
  it.each([
    ['a curl command', "curl --location 'https://pokeapi.co/api/v2/pokemon/ditto'", 'curl', true],
    ['a curl command copied with its prompt', '$ curl -X POST https://example.com', 'curl', true],
    ['a .http request', 'POST https://example.com/orders\nContent-Type: application/json\n\n{}', 'http', true],
    ['a Flowi message request file', 'id: 01M63D04BXES8ME7EM4FSWYBEJ\nname: Listen\nversions:\n  "1":\n    async:\n      protocol: amqp\n', 'flowi', true],
    ['a Flowi HTTP request file', 'name: Login\nversions:\n  "1":\n    method: POST\n    url: "{{env.base_url}}/v1/login"\n', 'flowi', true],
    ['a .http QUERY request', 'QUERY https://example.com/search\nContent-Type: application/json\n\n{}', 'http', true],
    ['a .http request after a comment', '# make an order\nGET {{base}}/orders HTTP/1.1\n', 'http', true],
    ['an OpenAPI document in JSON', '{"openapi": "3.1.0", "paths": {}}', 'openapi', false],
    ['a Swagger document in YAML', 'swagger: "2.0"\ninfo:\n  title: Shop', 'openapi', false],
    ['an AsyncAPI document', 'asyncapi: 3.0.0\nchannels: {}', 'asyncapi', false],
    ['a HAR file', '{"log": {"version": "1.2", "entries": []}}', 'har', false],
    ['a Postman collection', '{"info": {"_postman_id": "x", "name": "Shop"}, "item": []}', 'postman', false],
  ])('Given %s Then it is recognised', (_, text, id, fills) => {
    const got = recognise(text)

    expect(got).toMatchObject({ id, fills })
  })

  it.each([
    ['an address', 'https://pokeapi.co/api/v2/pokemon/ditto'],
    ['a template', '{{env.base_url}}/v1/login'],
    ['a method and an address on one line', 'GET https://example.com'],
    ['JSON that is no document', '{"name": "ditto"}'],
    ['text that is not JSON', '{not json'],
    ['nothing', '   '],
  ])('Given %s Then it pastes as text', (_, text) => {
    expect(recognise(text)).toBeNull()
  })
})

describe('paste anywhere in the window', () => {
  const curl = "curl -X POST https://api.example.com/orders -d '{}'"

  it('tells a place that takes text from one that does not', () => {
    expect(isEditable({ tagName: 'INPUT' })).toBe(true)
    expect(isEditable({ tagName: 'textarea' })).toBe(true)
    expect(isEditable({ tagName: 'DIV', isContentEditable: true })).toBe(true)
    expect(isEditable({ tagName: 'DIV', closest: (s: string) => (s.includes('.cm-editor') ? {} : null) })).toBe(true)
    expect(isEditable({ tagName: 'DIV', closest: () => null })).toBe(false)
    expect(isEditable(null)).toBe(false)
  })

  it('offers an import for a recognised paste on the canvas, sidebar or Home', () => {
    // Before: only the request address bar looked at a paste.
    expect(windowPaste({ tagName: 'DIV' }, curl, false)?.id).toBe('curl')
  })

  it('leaves a paste into a field, a handled one, plain text and a request file alone', () => {
    expect(windowPaste({ tagName: 'INPUT' }, curl, false)).toBeNull()
    expect(windowPaste({ tagName: 'DIV' }, curl, true)).toBeNull()
    expect(windowPaste({ tagName: 'DIV' }, 'hello', false)).toBeNull()
    expect(windowPaste({ tagName: 'DIV' }, 'versions:\n  - label: a\n    method: GET\n', false)).toBeNull()
  })

  it('is listened for on the window and opens the offer without a request', () => {
    expect(app).toContain('onpaste={onWindowPaste}')
    expect(app).toContain("app.pasteOffer = { node: '', kind, text, http: false, field: null }")
  })
})
