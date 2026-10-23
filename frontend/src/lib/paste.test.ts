import { describe, expect, it } from 'vitest'
import { recognise } from './paste'

describe('recognise', () => {
  it.each([
    ['a curl command', "curl --location 'https://pokeapi.co/api/v2/pokemon/ditto'", 'curl', true],
    ['a curl command copied with its prompt', '$ curl -X POST https://example.com', 'curl', true],
    ['a .http request', 'POST https://example.com/orders\nContent-Type: application/json\n\n{}', 'http', true],
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
