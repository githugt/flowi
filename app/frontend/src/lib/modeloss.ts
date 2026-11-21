import type { AsyncBlock } from './types'

/**
 * What switching a message request to `mode` would throw away, in words, or '' when nothing written is lost.
 * Before, "Receive" deleted the written message and "Send" deleted the count, timeout and filter, with no question.
 */
export function modeLoss(block: AsyncBlock | undefined, mode: string): string {
  if (!block) return ''
  const lost: string[] = []
  if (mode === 'receive' && block.Send) {
    const b = block.Send.Body
    if (b.JSON.trim() || b.Text.trim() || b.File || b.Form?.length || b.Multipart?.length) lost.push('the message you wrote')
    if (block.Send.Headers?.length) lost.push('its headers')
  }
  if (mode === 'send' && block.Receive) {
    const r = block.Receive
    if (r.Count > 1 || r.Timeout > 0 || r.Match.trim()) lost.push('the count, time limit and filter of what it waits for')
  }
  return lost.join(' and ')
}
