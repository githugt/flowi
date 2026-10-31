// What the step form's fields share: the completions and vocabulary a template or expression field offers, and
// which help page a parameter points at. The side inspector and the card rows both use them.
import { FlowService, SystemService } from '../../bindings/github.com/githugt/flowi/app/internal/bridge'
import type { Item } from './exprcomplete'

/** What a step's field can read depends on what the field is. */
export function helpFor(type: string, p: { name: string; kind: string }): string {
  if (p.kind === 'expr') return 'flow.check'
  if (type === 'batch' && p.name === 'over') return 'flow.collection'
  return 'flow.template'
}

/** Examples of the expressions a step's check, until and when take: true or false, no braces. */
export const exprHint = 'Examples: nodes.login.status == 200 · vars.count > 3 · response.body.token != empty.'

/** Completes the template being typed in a field of the flow `node`. */
export const completeFor = (node: string) => (text: string, caret: number) => FlowService.Complete(node, text, caret) as unknown as Promise<any>

/**
 * What a bare expression (a test, a "run when") can name: the context's roots, functions and keywords, then the
 * real names of the step's surroundings, such as nodes.login.body.token.
 */
export const vocabularyFor = (node: string, context: string) => async (): Promise<Item[]> => {
  const [roots, names] = await Promise.all([SystemService.ExpressionVocabulary(context), FlowService.ExpressionNames(node)])
  return [...(names as Item[]), ...(roots as Item[])]
}
