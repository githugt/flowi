import { RequestService, SystemService } from '../../bindings/github.com/githugt/flowi/internal/bridge'
import { bodyPaths, type Item } from './exprcomplete'
import type { ResponseView } from './types'

const source = 'from the last response'

function parse(text: string): unknown {
  try {
    return JSON.parse(text)
  } catch {
    return undefined // not JSON: no fields to offer
  }
}

/**
 * The body fields known from what the request last returned: the parsed body of an HTTP response,
 * or the JSON of the first message received. They are the paths a check or a filter can read.
 */
export function responsePaths(response: ResponseView | null | undefined, kind: string, asMessages: boolean): Item[] {
  if (!response) return []
  if (kind === 'http') return bodyPaths(response.bodyJson, 'body', source)
  const first = response.messages?.find((m) => m.direction !== 'sent') ?? response.messages?.[0]
  const body = first && parse(first.body)
  if (body === undefined || body === null) return []
  return asMessages ? [...bodyPaths(body, 'body', source), ...bodyPaths(body, 'messages[0].body', source)] : bodyPaths(body, 'body', source)
}

/**
 * What a check or filter on a request can name: the body fields of its last response, the
 * environment and its own variables, then the roots, functions and keywords of the context.
 */
export function requestVocabulary(context: string, node: string, response: ResponseView | null | undefined, kind: string) {
  return async (): Promise<Item[]> => {
    const [roots, names] = await Promise.all([SystemService.ExpressionVocabulary(context), RequestService.ExpressionNames(node)])
    return [...responsePaths(response, kind, context !== 'message.filter'), ...(names as Item[]), ...(roots as Item[])]
  }
}
